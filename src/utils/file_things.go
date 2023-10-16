package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"log/slog"
)

func LoadFromFile(filePath string, key []byte) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		slog.Error(err.Error())
		return ""
	}
	if strings.HasPrefix(string(data), "0x") {
		data2 := strings.TrimPrefix(string(data), "0x")
		writeToFile(filePath, data2, key)

		return data2
	}
	txtPlain, _ := Decrypt(string(data), key)
	return txtPlain
}

func writeToFile(filePath string, txt string, key []byte) {
	fmt.Println("storing file")
	txtEnc, _ := Encrypt(txt, key)
	data := []byte(txtEnc)
	err := os.WriteFile(filePath, data, 0644)
	if err != nil {
		slog.Error("Error writing file:" + err.Error())
	}
}

func Encrypt(plainText string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	encrypted := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return hex.EncodeToString(encrypted), nil
}

func Decrypt(encryptedText string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	encrypted, err := hex.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, encrypted := encrypted[:nonceSize], encrypted[nonceSize:]
	plainText, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}
