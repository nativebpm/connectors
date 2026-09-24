package manticore_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/nativebpm/connectors/manticore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMySQLPeer starts a minimal mock TCP server that mimics Manticore's MySQL wire protocol.
type mockMySQLPeer struct {
	listener net.Listener
	addr     string
	queries  []string
	mu       sync.Mutex
	closed   bool
}

func startMockPeerListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func startMockPeer(t *testing.T) *mockMySQLPeer {
	l, err := startMockPeerListener()
	require.NoError(t, err)

	peer := &mockMySQLPeer{
		listener: l,
		addr:     l.Addr().String(),
	}

	go peer.serve()
	return peer
}

func (p *mockMySQLPeer) serve() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.handleConn(conn)
	}
}

func (p *mockMySQLPeer) handleConn(conn net.Conn) {
	defer conn.Close()

	// 1. Send Handshake Initialization Packet (v10)
	var buf bytes.Buffer
	buf.WriteByte(10) // Protocol 10
	buf.WriteString("5.5.5-manticore-mock\x00")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(1)) // Connection ID
	buf.WriteString("12345678\x00")                       // Auth-plugin-data-part-1
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0xf7ff)) // Lower capabilities (with CLIENT_PROTOCOL_41)
	buf.WriteByte(33)                                      // Character set utf8_general_ci
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2)) // Status flags
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0x0008)) // Upper capabilities (CLIENT_PLUGIN_AUTH)
	buf.WriteByte(21)                                      // Length of auth plugin data
	buf.Write(make([]byte, 10))                            // Reserved 10 bytes
	buf.WriteString("123456789012\x00")                   // Auth-plugin-data-part-2
	buf.WriteString("mysql_native_password\x00")

	payload := buf.Bytes()
	header := make([]byte, 4)
	header[0] = byte(len(payload))
	header[1] = byte(len(payload) >> 8)
	header[2] = byte(len(payload) >> 16)
	header[3] = 0 // Sequence ID 0

	_, _ = conn.Write(append(header, payload...))

	// 2. Read Client Handshake Response
	handshakeRespHeader := make([]byte, 4)
	if _, err := conn.Read(handshakeRespHeader); err != nil {
		return
	}
	respLen := int(handshakeRespHeader[0]) | int(handshakeRespHeader[1])<<8 | int(handshakeRespHeader[2])<<16
	respPayload := make([]byte, respLen)
	if _, err := conn.Read(respPayload); err != nil {
		return
	}

	// 3. Send OK packet for Authentication
	// Packet: 7 bytes: len=7, seq=2, 0x00 (OK), 0 affected, 0 last_insert_id, 0 status, 0 warnings
	okPacket := []byte{0x07, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
	if _, err := conn.Write(okPacket); err != nil {
		return
	}

	// 4. Command Loop
	cmdHeader := make([]byte, 4)
	for {
		if _, err := conn.Read(cmdHeader); err != nil {
			return
		}
		cmdLen := int(cmdHeader[0]) | int(cmdHeader[1])<<8 | int(cmdHeader[2])<<16
		cmdPayload := make([]byte, cmdLen)
		if _, err := conn.Read(cmdPayload); err != nil {
			return
		}

		if len(cmdPayload) > 0 && cmdPayload[0] == 0x03 { // COM_QUERY
			query := string(cmdPayload[1:])
			p.mu.Lock()
			p.queries = append(p.queries, query)
			p.mu.Unlock()

			// Send OK packet
			reply := []byte{0x07, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
			if _, err := conn.Write(reply); err != nil {
				return
			}
		} else if len(cmdPayload) > 0 && cmdPayload[0] == 0x01 { // COM_QUIT
			return
		} else {
			// Reply OK for Pings / other commands
			reply := []byte{0x07, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
			if _, err := conn.Write(reply); err != nil {
				return
			}
		}
	}
}

func (p *mockMySQLPeer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		_ = p.listener.Close()
	}
}

func (p *mockMySQLPeer) Queries() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	copied := make([]string, len(p.queries))
	copy(copied, p.queries)
	return copied
}

func TestManticore_PeersBroadcast(t *testing.T) {
	peer1 := startMockPeer(t)
	defer peer1.Close()

	peer2 := startMockPeer(t)
	defer peer2.Close()

	cfg := manticore.Config{
		Peers:           []string{peer1.addr, peer2.addr},
		Timeout:         2 * time.Second,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		RetentionPeriod: 24 * time.Hour,
		MaxRowsCapacity: 10000,
	}

	ctx := context.Background()
	client, err := manticore.NewClient(ctx, cfg)
	require.NoError(t, err)
	defer client.Close()

	// 1. Verify Ping on both peers
	err = client.Ping(ctx)
	require.NoError(t, err)

	// 2. Broadcast UpsertInstance across peers
	inst := &manticore.ProcessInstance{
		ID:         10042,
		ProcessKey: "order_approval",
		Version:    1,
		Status:     "RUNNING",
		TenantID:   1,
		Assignee:   "operator_1",
		StartTime:  time.Now(),
		DurationMs: 0,
		Amount:     15000.50,
	}

	err = client.UpsertInstance(ctx, inst)
	require.NoError(t, err)

	// Verify both peers received the query
	time.Sleep(50 * time.Millisecond)
	q1 := peer1.Queries()
	q2 := peer2.Queries()

	require.GreaterOrEqual(t, len(q1), 1)
	require.GreaterOrEqual(t, len(q2), 1)
	assert.Contains(t, q1[len(q1)-1], "REPLACE INTO process_instances")
	assert.Contains(t, q2[len(q2)-1], "REPLACE INTO process_instances")

	// 3. In-place UpdateStatus across peers
	err = client.UpdateStatus(ctx, 10042, "COMPLETED", 1520)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	q1 = peer1.Queries()
	q2 = peer2.Queries()
	assert.Contains(t, q1[len(q1)-1], "UPDATE process_instances SET status")
	assert.Contains(t, q2[len(q2)-1], "UPDATE process_instances SET status")

	// 4. Bounded Retention Pruning across peers
	err = client.PruneExpired(ctx, time.Now().Add(-24*time.Hour))
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	q1 = peer1.Queries()
	q2 = peer2.Queries()
	assert.Contains(t, q1[len(q1)-1], "DELETE FROM process_instances WHERE status = 'COMPLETED'")
	assert.Contains(t, q2[len(q2)-1], "DELETE FROM process_instances WHERE status = 'COMPLETED'")
}
