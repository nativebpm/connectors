package temporal

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	common "go.temporal.io/api/common/v1"
)

func TestCryptCodec_EncodeDecode(t *testing.T) {
	// Initialize key
	passphrase := "my_test_secure_passphrase"
	hash := sha256.Sum256([]byte(passphrase))
	key := hash[:]

	codec, err := NewCryptCodec(key)
	assert.NoError(t, err)

	// Create test payload
	originalPayload := &common.Payload{
		Metadata: map[string][]byte{
			"encoding": []byte("json/plain"),
			"custom":   []byte("value"),
		},
		Data: []byte(`{"message": "secret business data"}`),
	}

	payloads := []*common.Payload{originalPayload}

	// 1. Encrypt (Encode)
	encodedPayloads, err := codec.Encode(payloads)
	assert.NoError(t, err)
	assert.Len(t, encodedPayloads, 1)

	encoded := encodedPayloads[0]
	assert.NotNil(t, encoded)
	// Verify metadata is changed to encrypted
	assert.Equal(t, []byte("binary/encrypted"), encoded.Metadata["encoding"])
	assert.NotEqual(t, originalPayload.Data, encoded.Data)
	assert.Nil(t, encoded.Metadata["custom"]) // Original metadata should be hidden inside the encrypted block

	// 2. Decrypt (Decode)
	decodedPayloads, err := codec.Decode(encodedPayloads)
	assert.NoError(t, err)
	assert.Len(t, decodedPayloads, 1)

	decoded := decodedPayloads[0]
	assert.NotNil(t, decoded)
	// Verify match with original object
	assert.Equal(t, []byte("json/plain"), decoded.Metadata["encoding"])
	assert.Equal(t, []byte("value"), decoded.Metadata["custom"])
	assert.Equal(t, originalPayload.Data, decoded.Data)
}
