package chshare

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/jpillora/sizestr"
	"golang.org/x/crypto/ssh"
)

// GenerateKey generates a keypair to use for the SSH server end, using
// an optional seed that will produce the same keypair every time. If
// seed is "", a random key will be generated.
func GenerateKey(seed string) ([]byte, error) {
	var r io.Reader
	if seed == "" {
		r = rand.Reader
	} else {
		r = NewDetermRand([]byte(seed))
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), r)
	if err != nil {
		return nil, err
	}
	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("Unable to marshal ECDSA private key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}), nil
}

// FingerprintKey returns a standard fingerprint hash string for an SSH
// public key, which clients can use to authenticate the SSH server.
func FingerprintKey(k ssh.PublicKey) string {
	bytes := md5.Sum(k.Marshal())
	strbytes := make([]string, len(bytes))
	for i, b := range bytes {
		strbytes[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(strbytes, ":")
}

// HandleTCPStream handles a new ssh.Conn from a remote Stub that needs to Dial
// to a local network resource and pipe between them. Returns when the connection
// is complete. src will be closed before returning.
func HandleTCPStream(l Logger, connStats *ConnStats, src io.ReadWriteCloser, remote string) {
	dst, err := net.Dial("tcp", remote)
	if err != nil {
		l.DLogf("Remote failed (%s)", err)
		src.Close()
		return
	}
	connStats.Open()
	l.DLogf("%s: Open", connStats)
	s, r := Pipe(src, dst)
	connStats.Close()
	l.DLogf("%s: Close (sent %s received %s)", connStats, sizestr.ToString(s), sizestr.ToString(r))
}
