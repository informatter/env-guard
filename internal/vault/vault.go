package vault

import "errors"

var (
	// ErrWrongPassword is returned when the provided master password is incorrect.
	ErrWrongPassword = errors.New("wrong master password")

	// ErrVaultExists is returned when attempting to create a vault that already exists.
	ErrVaultExists = errors.New("vault already exists")

	// ErrNoVault is returned when attempting to open a vault that does not exist.
	ErrNoVault = errors.New("vault not found; run env-guard init first")

	// ErrVaultLocked is returned when attempting to access secrets while the vault is locked.
	ErrVaultLocked = errors.New("vault is locked")

	// ErrSecretNotFound is returned when querying a secret key that does not exist in the project.
	ErrSecretNotFound = errors.New("secret not found")

	// ErrSecretNotSet is returned when querying a secret key that has no value assigned yet.
	ErrSecretNotSet = errors.New("secret has no value set")

	// ErrProjectNotFound is returned when referencing a project that does not exist.
	ErrProjectNotFound = errors.New("project not found")
)

// AccessEntry represents a single entry in the daemon access log.
type AccessEntry struct {
	ID            int64  `json:"id"`
	Timestamp     int64  `json:"timestamp"`
	AppName       string `json:"app_name"`
	ProcessPath   string `json:"process_path"`
	KeysRequested string `json:"keys_requested"`
	PID           int    `json:"pid"`
}

// Vault defines the interface for encrypted secret storage.
// Implementations must encrypt all secret values at rest and
// require a master password to unlock.
type Vault interface {
	// Create initialises a new encrypted vault with the given master password.
	// Returns ErrVaultExists if a vault already exists at the configured path.
	Create(password string) error

	// Open unlocks an existing vault with the given master password.
	// Returns ErrNoVault if no vault exists, ErrWrongPassword if the password is incorrect.
	Open(password string) error

	// Lock clears the decryption key from memory, preventing further access.
	// All subsequent operations return ErrVaultLocked until Open is called again.
	Lock() error

	// Close clears the decryption key and closes the database connection.
	Close() error

	// IsOpen reports whether the vault is currently unlocked and ready for use.
	IsOpen() bool

	// CreateProject creates a new project (namespace for secrets).
	// Duplicate project names are silently ignored.
	CreateProject(name string) error

	// Projects returns the names of all projects in the vault.
	Projects() ([]string, error)

	// InitSecret creates a secret key entry with no value.
	// Used during initial setup to define which secrets an app requires.
	// Returns ErrSecretNotSet if GetSecret is called before a value is provided.
	InitSecret(project, key string) error

	// SetSecret encrypts and stores a secret value for the given project and key.
	// Creates the entry if it does not already exist (upsert semantics).
	SetSecret(project, key, value string) error

	// GetSecret decrypts and returns the value for the given project and key.
	// Returns ErrSecretNotFound if the key does not exist.
	// Returns ErrSecretNotSet if the key exists but has no value.
	GetSecret(project, key string) (string, error)

	// GetSecrets returns all set secrets for a project as a key-value map.
	// Secrets that have been initialised but not yet given a value are omitted.
	GetSecrets(project string) (map[string]string, error)

	// SecretKeys returns all secret keys defined for a project, sorted alphabetically.
	SecretKeys(project string) ([]string, error)

	// LogAccess records an API access in the audit log.
	LogAccess(appName, processPath, keysRequested string, pid int) error

	// AccessLog returns the most recent 100 access log entries, newest first.
	AccessLog() ([]AccessEntry, error)
}
