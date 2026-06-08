package vault

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const vaultDir = ".env-guard"
const vaultFile = "vault.db"

var schema = []string{
	`CREATE TABLE IF NOT EXISTS key_slots (
		id INTEGER PRIMARY KEY,
		purpose TEXT NOT NULL CHECK(purpose IN ('master', 'recovery')),
		salt BLOB NOT NULL,
		nonce BLOB NOT NULL,
		encrypted_vault_key BLOB NOT NULL,
		argon2_time INTEGER NOT NULL DEFAULT 2,
		argon2_memory INTEGER NOT NULL DEFAULT 19456,
		argon2_threads INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE
	)`,
	`CREATE TABLE IF NOT EXISTS secrets (
		id INTEGER PRIMARY KEY,
		project_id INTEGER NOT NULL REFERENCES projects(id),
		key TEXT NOT NULL,
		encrypted_value BLOB,
		nonce BLOB,
		UNIQUE(project_id, key)
	)`,
	`CREATE TABLE IF NOT EXISTS access_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		app_name TEXT NOT NULL,
		process_path TEXT NOT NULL,
		keys_requested TEXT NOT NULL,
		pid INTEGER NOT NULL
	)`,
}

// SQLiteVault is an encrypted secret store backed by SQLite.
// Secret values are encrypted with AES-256-GCM before storage.
// The vault key itself is encrypted with a key derived from the
// master password via Argon2id and stored in a key slot.
type SQLiteVault struct {
	mu       sync.Mutex
	db       *sql.DB
	dbPath   string
	vaultKey []byte
	open     bool
}

type keySlotRow struct {
	id               int64
	purpose          string
	salt             []byte
	nonce            []byte
	encryptedVaultKey []byte
	argon2Time       int
	argon2Memory     int
	argon2Threads    int
}

// DefaultVaultPath returns the default path for the vault database:
// $HOME/.env-guard/vault.db
func DefaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, vaultDir, vaultFile), nil
}

// New creates a new SQLiteVault targeting the given database path.
// The vault is not created or opened until Create or Open is called.
func New(dbPath string) *SQLiteVault {
	return &SQLiteVault{dbPath: dbPath}
}

func (v *SQLiteVault) Create(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db != nil {
		return ErrVaultExists
	}

	if err := os.MkdirAll(filepath.Dir(v.dbPath), 0o700); err != nil {
		return fmt.Errorf("creating vault directory: %w", err)
	}

	if _, err := os.Stat(v.dbPath); err == nil {
		return ErrVaultExists
	}

	db, err := sql.Open("sqlite", v.dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	if err := applyPragmas(db); err != nil {
		db.Close()
		return err
	}

	if err := createTables(db); err != nil {
		db.Close()
		return err
	}

	vaultKey, err := generateKey()
	if err != nil {
		db.Close()
		return fmt.Errorf("generating vault key: %w", err)
	}

	salt, err := generateSalt()
	if err != nil {
		db.Close()
		return fmt.Errorf("generating salt: %w", err)
	}

	derivedKey := deriveKey(password, salt)
	encryptedKey, nonce, err := encrypt(vaultKey, derivedKey)
	if err != nil {
		db.Close()
		return fmt.Errorf("encrypting vault key: %w", err)
	}

	_, err = db.Exec(
		`INSERT INTO key_slots (purpose, salt, nonce, encrypted_vault_key, argon2_time, argon2_memory, argon2_threads)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"master", salt, nonce, encryptedKey, argon2Time, argon2Memory, argon2Thread,
	)
	if err != nil {
		db.Close()
		return fmt.Errorf("inserting key slot: %w", err)
	}

	v.db = db
	v.vaultKey = vaultKey
	v.open = true
	return nil
}

func (v *SQLiteVault) Open(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db != nil {
		return nil
	}

	if _, err := os.Stat(v.dbPath); os.IsNotExist(err) {
		return ErrNoVault
	}

	db, err := sql.Open("sqlite", v.dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	if err := applyPragmas(db); err != nil {
		db.Close()
		return err
	}

	slot, err := readKeySlot(db, "master")
	if err != nil {
		db.Close()
		return err
	}

	derivedKey := deriveKey(password, slot.salt)
	vaultKey, err := decrypt(slot.encryptedVaultKey, derivedKey, slot.nonce)
	if err != nil {
		db.Close()
		return ErrWrongPassword
	}

	v.db = db
	v.vaultKey = vaultKey
	v.open = true
	return nil
}

func (v *SQLiteVault) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.open {
		return nil
	}

	for i := range v.vaultKey {
		v.vaultKey[i] = 0
	}
	v.vaultKey = nil
	v.open = false
	return nil
}

func (v *SQLiteVault) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.vaultKey != nil {
		for i := range v.vaultKey {
			v.vaultKey[i] = 0
		}
		v.vaultKey = nil
	}
	v.open = false

	if v.db != nil {
		return v.db.Close()
	}
	return nil
}

func (v *SQLiteVault) IsOpen() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.open
}

func (v *SQLiteVault) requireOpen() error {
	if !v.open || v.vaultKey == nil {
		return ErrVaultLocked
	}
	return nil
}

func (v *SQLiteVault) CreateProject(name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return err
	}

	_, err := v.db.Exec(`INSERT INTO projects (name) VALUES (?)`, name)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil
		}
		return fmt.Errorf("creating project: %w", err)
	}
	return nil
}

func (v *SQLiteVault) Projects() ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return nil, err
	}

	rows, err := v.db.Query(`SELECT name FROM projects ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (v *SQLiteVault) projectID(name string) (int64, error) {
	var id int64
	err := v.db.QueryRow(`SELECT id FROM projects WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, ErrProjectNotFound
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (v *SQLiteVault) InitSecret(project, key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return err
	}

	pid, err := v.projectID(project)
	if err != nil {
		return err
	}

	_, err = v.db.Exec(
		`INSERT INTO secrets (project_id, key, encrypted_value, nonce) VALUES (?, ?, NULL, NULL)
		 ON CONFLICT(project_id, key) DO NOTHING`,
		pid, key,
	)
	return err
}

func (v *SQLiteVault) SetSecret(project, key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return err
	}

	pid, err := v.projectID(project)
	if err != nil {
		return err
	}

	nonce, err := generateNonce()
	if err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}

	encryptedValue, err := encryptSecretValue([]byte(value), v.vaultKey, nonce)
	if err != nil {
		return fmt.Errorf("encrypting secret: %w", err)
	}

	_, err = v.db.Exec(
		`INSERT INTO secrets (project_id, key, encrypted_value, nonce) VALUES (?, ?, ?, ?)
		 ON CONFLICT(project_id, key) DO UPDATE SET encrypted_value = excluded.encrypted_value, nonce = excluded.nonce`,
		pid, key, encryptedValue, nonce,
	)
	return err
}

func (v *SQLiteVault) GetSecret(project, key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return "", err
	}

	pid, err := v.projectID(project)
	if err != nil {
		return "", err
	}

	var encryptedValue, nonce []byte
	err = v.db.QueryRow(
		`SELECT encrypted_value, nonce FROM secrets WHERE project_id = ? AND key = ?`,
		pid, key,
	).Scan(&encryptedValue, &nonce)

	if err == sql.ErrNoRows {
		return "", ErrSecretNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading secret: %w", err)
	}

	if encryptedValue == nil {
		return "", ErrSecretNotSet
	}

	plaintext, err := decryptSecretValue(encryptedValue, v.vaultKey, nonce)
	if err != nil {
		return "", fmt.Errorf("decrypting secret: %w", err)
	}

	return string(plaintext), nil
}

func (v *SQLiteVault) GetSecrets(project string) (map[string]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return nil, err
	}

	pid, err := v.projectID(project)
	if err != nil {
		return nil, err
	}

	rows, err := v.db.Query(
		`SELECT key, encrypted_value, nonce FROM secrets WHERE project_id = ? ORDER BY key`,
		pid,
	)
	if err != nil {
		return nil, fmt.Errorf("reading secrets: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key string
		var encryptedValue, nonce []byte
		if err := rows.Scan(&key, &encryptedValue, &nonce); err != nil {
			return nil, fmt.Errorf("scanning secret: %w", err)
		}
		if encryptedValue == nil {
			continue
		}
		plaintext, err := decryptSecretValue(encryptedValue, v.vaultKey, nonce)
		if err != nil {
			return nil, fmt.Errorf("decrypting secret %q: %w", key, err)
		}
		result[key] = string(plaintext)
	}
	return result, rows.Err()
}

func (v *SQLiteVault) SecretKeys(project string) ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return nil, err
	}

	pid, err := v.projectID(project)
	if err != nil {
		return nil, err
	}

	rows, err := v.db.Query(
		`SELECT key FROM secrets WHERE project_id = ? ORDER BY key`,
		pid,
	)
	if err != nil {
		return nil, fmt.Errorf("listing secret keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scanning secret key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (v *SQLiteVault) LogAccess(appName, processPath, keysRequested string, pid int) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.open {
		return ErrVaultLocked
	}

	keysJSON, err := json.Marshal(keysRequested)
	if err != nil {
		keysJSON = []byte(`"` + keysRequested + `"`)
	}

	_, err = v.db.Exec(
		`INSERT INTO access_log (timestamp, app_name, process_path, keys_requested, pid) VALUES (?, ?, ?, ?, ?)`,
		time.Now().Unix(), appName, processPath, string(keysJSON), pid,
	)
	return err
}

func (v *SQLiteVault) AccessLog() ([]AccessEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.requireOpen(); err != nil {
		return nil, err
	}

	rows, err := v.db.Query(
		`SELECT id, timestamp, app_name, process_path, keys_requested, pid FROM access_log ORDER BY timestamp DESC LIMIT 100`,
	)
	if err != nil {
		return nil, fmt.Errorf("reading access log: %w", err)
	}
	defer rows.Close()

	var entries []AccessEntry
	for rows.Next() {
		var e AccessEntry
		var keysJSON string
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.AppName, &e.ProcessPath, &keysJSON, &e.PID); err != nil {
			return nil, fmt.Errorf("scanning access log: %w", err)
		}
		json.Unmarshal([]byte(keysJSON), &e.KeysRequested)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("setting pragma %q: %w", p, err)
		}
	}
	return nil
}

func createTables(db *sql.DB) error {
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("creating table: %w", err)
		}
	}
	return nil
}

func readKeySlot(db *sql.DB, purpose string) (*keySlotRow, error) {
	row := db.QueryRow(
		`SELECT id, purpose, salt, nonce, encrypted_vault_key, argon2_time, argon2_memory, argon2_threads
		 FROM key_slots WHERE purpose = ? LIMIT 1`,
		purpose,
	)

	var s keySlotRow
	err := row.Scan(&s.id, &s.purpose, &s.salt, &s.nonce, &s.encryptedVaultKey, &s.argon2Time, &s.argon2Memory, &s.argon2Threads)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no key slot found for purpose %q", purpose)
	}
	if err != nil {
		return nil, fmt.Errorf("reading key slot: %w", err)
	}
	return &s, nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE")
}
