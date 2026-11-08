package models

import (
	"crypto/rand"
	"encoding/hex"
)

// NuevoID genera un identificador único e impredecible de 128 bits en
// hexadecimal. Se usa como nombre del archivo en disco, así que nunca
// depende de lo que envíe el usuario.
func NuevoID() (string, error) {
	bytesAleatorios := make([]byte, 16)
	if _, err := rand.Read(bytesAleatorios); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytesAleatorios), nil
}
