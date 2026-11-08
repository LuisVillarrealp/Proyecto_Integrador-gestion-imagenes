package red

import "testing"

func TestEsLoopback(t *testing.T) {
	casos := map[string]bool{
		"localhost:8443": true, "127.0.0.1:8443": true, "[::1]:8443": true, "127.0.0.1": true,
		":8443": false, "0.0.0.0:8443": false, "10.10.0.50:8443": false, "": false, "ejemplo.com:443": false,
	}
	for addr, esperado := range casos {
		if got := EsLoopback(addr); got != esperado {
			t.Errorf("EsLoopback(%q) = %v, se esperaba %v", addr, got, esperado)
		}
	}
}
