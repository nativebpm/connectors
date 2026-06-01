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

// CryptCodec реализует интерфейс converter.PayloadCodec для шифрования данных.
type CryptCodec struct {
	key []byte
}

// NewCryptCodec создает новый экземпляр CryptCodec с 16, 24 или 32-байтным ключом для AES.
func NewCryptCodec(key []byte) (*CryptCodec, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("key length must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256")
	}
	return &CryptCodec{key: key}, nil
}

// Encode шифрует переданные полезные нагрузки (payloads) перед отправкой на сервер.
func (c *CryptCodec) Encode(payloads []*common.Payload) ([]*common.Payload, error) {
	result := make([]*common.Payload, len(payloads))
	for i, p := range payloads {
		if p == nil {
			continue
		}

		// Сериализуем оригинальный Payload полностью (включая его метаданные)
		payloadBytes, err := p.Marshal()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}

		// Инициализируем шифр AES-GCM
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

		// Создаем новый Payload с зашифрованными данными
		result[i] = &common.Payload{
			Metadata: map[string][]byte{
				"encoding": []byte("binary/encrypted"),
			},
			Data: encryptedData,
		}
	}
	return result, nil
}

// Decode расшифровывает полезные нагрузки, пришедшие от сервера.
func (c *CryptCodec) Decode(payloads []*common.Payload) ([]*common.Payload, error) {
	result := make([]*common.Payload, len(payloads))
	for i, p := range payloads {
		if p == nil {
			continue
		}

		// Декодируем только зашифрованные полезные нагрузки
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

		// Восстанавливаем оригинальный Payload
		originalPayload := &common.Payload{}
		if err := originalPayload.Unmarshal(decryptedBytes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal decrypted payload: %w", err)
		}

		result[i] = originalPayload
	}
	return result, nil
}

// GetEncryptingDataConverter оборачивает дефолтный DataConverter шифрующим кодеком.
func GetEncryptingDataConverter(key []byte) (converter.DataConverter, error) {
	codec, err := NewCryptCodec(key)
	if err != nil {
		return nil, err
	}
	return converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), codec), nil
}
