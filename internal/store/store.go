package store

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/scrypt"
)

type Entry struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Notes    string `json:"notes,omitempty"`
}

type DB struct {
	Entries []Entry `json:"entries"`
}

// File format: [version:1][salt:32][nonce:24][ciphertext+tag:N]
// version byte is always 2. scrypt(N=131072), XChaCha20-Poly1305, salt as AAD.
const (
	formatVersion = byte(2)

	saltLen = 32
	keyLen  = 32

	scryptN = 131072
	scryptR = 8
	scryptP = 1
)

func deriveKey(password string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, keyLen)
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func encryptDB(db *DB, password string) ([]byte, error) {
	plain, err := json.Marshal(db)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plain)

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key, err := deriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// layout: [version][salt][nonce][ciphertext+tag]
	// salt is passed as AAD so any tampering with it is detected.
	out := make([]byte, 0, 1+saltLen+len(nonce)+len(plain)+aead.Overhead())
	out = append(out, formatVersion)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plain, salt)
	return out, nil
}

func decryptDB(blob []byte, password string) (*DB, error) {
	if len(blob) < 1+saltLen+1 {
		return nil, errors.New("data too short")
	}

	if blob[0] != formatVersion {
		return nil, errors.New("unsupported database version")
	}

	rest := blob[1:]
	salt := rest[:saltLen]
	rest = rest[saltLen:]

	key, err := deriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	if len(rest) < aead.NonceSize() {
		return nil, errors.New("data too short")
	}

	nonce := rest[:aead.NonceSize()]
	ciphertext := rest[aead.NonceSize():]

	plain, err := aead.Open(nil, nonce, ciphertext, salt)
	if err != nil {
		return nil, errors.New("wrong password or corrupted data")
	}
	defer zeroBytes(plain)

	var dbOut DB
	if err := json.Unmarshal(plain, &dbOut); err != nil {
		return nil, err
	}
	return &dbOut, nil
}

// Load reads and decrypts the db file. Returns nil (no error) if the file does not exist yet.
func Load(path, password string) (*DB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return decryptDB(data, password)
}

// Save atomically encrypts and writes the database.
// It writes to a temp file in the same directory, fsyncs, then renames over
// the target so a crash mid-write never leaves a partial/corrupt file.
func Save(db *DB, path, password string) error {
	data, err := encryptDB(db, password)
	if err != nil {
		return err
	}
	defer zeroBytes(data)

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".secretly-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	ok = true
	return nil
}

// ConstantTimeEq compares two strings in constant time.
func ConstantTimeEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// AcquireLock obtains an exclusive advisory lock stored in the OS temp dir,
// named after a hash of dbPath so it is always local and never cloud-synced.
// The returned *os.File must remain open for the lifetime of the process;
// closing it (or process exit) automatically releases the lock.
// Returns an error immediately if another instance already holds the lock.
func AcquireLock(dbPath string) (*os.File, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		abs = dbPath
	}
	sum := md5.Sum([]byte(abs))
	lockPath := filepath.Join(os.TempDir(), "secretly-"+hex.EncodeToString(sum[:])+".lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, errors.New("database is locked by another instance of secretly")
		}
		return nil, err
	}
	return f, nil
}
