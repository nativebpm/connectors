package temporal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	common "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// CryptCodec implements converter.PayloadCodec interface to encrypt/decrypt payloads.
type CryptCodec struct {
	key []byte
}

// NewCryptCodec creates a new CryptCodec instance with 16, 24, or 32-byte key for AES encryption.
func NewCryptCodec(key []byte) (*CryptCodec, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("key length must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256")
	}
	return &CryptCodec{key: key}, nil
}

// Encode encrypts payloads before sending them to the Temporal server.
func (c *CryptCodec) Encode(payloads []*common.Payload) ([]*common.Payload, error) {
	result := make([]*common.Payload, len(payloads))
	for i, p := range payloads {
		if p == nil {
			continue
		}

		// Marshal the original Payload completely (including its metadata)
		payloadBytes, err := p.Marshal()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}

		// Initialize AES-GCM cipher
		block, err := aes.NewCipher(c.key)
		if err != nil {
			return nil, err
		}

		aesGCM, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}

		nonce := make([]byte, aesGCM.NonceSize())
		if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
			return nil, err
		}

		encryptedData := aesGCM.Seal(nonce, nonce, payloadBytes, nil)

		// Create new Payload with encrypted data
		result[i] = &common.Payload{
			Metadata: map[string][]byte{
				"encoding": []byte("binary/encrypted"),
			},
			Data: encryptedData,
		}
	}
	return result, nil
}

// Decode decrypts payloads arriving from the Temporal server.
func (c *CryptCodec) Decode(payloads []*common.Payload) ([]*common.Payload, error) {
	result := make([]*common.Payload, len(payloads))
	for i, p := range payloads {
		if p == nil {
			continue
		}

		// Decode only encrypted payloads
		if string(p.Metadata["encoding"]) != "binary/encrypted" {
			result[i] = p
			continue
		}

		encryptedData := p.GetData()
		block, err := aes.NewCipher(c.key)
		if err != nil {
			return nil, err
		}

		aesGCM, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}

		nonceSize := aesGCM.NonceSize()
		if len(encryptedData) < nonceSize {
			return nil, errors.New("ciphertext too short")
		}

		nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
		decryptedBytes, err := aesGCM.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt payload: %w", err)
		}

		// Restore the original Payload
		originalPayload := &common.Payload{}
		if err := originalPayload.Unmarshal(decryptedBytes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal decrypted payload: %w", err)
		}

		result[i] = originalPayload
	}
	return result, nil
}

// GetEncryptingDataConverter wraps the default DataConverter with the encrypting codec.
func GetEncryptingDataConverter(key []byte) (converter.DataConverter, error) {
	codec, err := NewCryptCodec(key)
	if err != nil {
		return nil, err
	}
	return converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), codec), nil
}
