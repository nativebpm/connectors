# Cryptenv

`cryptenv` is a lightweight, secure, and reusable Go module designed for loading and managing sensitive environment secrets. It ensures that credentials are never stored in cleartext on disk or persistently in memory.

## Features

- **Memory-Safe Secret Storage**: All loaded secrets are stored in memory encrypted with AES-256-GCM. Decryption occurs on-the-fly *only* when a value is actively requested.
- **Pure Go Age Decryption**: Supports decrypting `age` encrypted files natively in Go (symmetric scrypt passphrase and asymmetric key-based). No external command line binary (like `gpg` or `age`) is required on the server.
- **Flexible Struct Loading**: Built-in struct tag parser supporting `env` and `envDefault` tags (similar to `caarlos0/env`).
- **YAML Unmarshaling**: Unmarshal YAML strings inside encrypted secrets directly into Go structs.
- **Third-Party Integrations**: Easily convert secrets to a `map[string]string` for integration with other configuration libraries.

---

## Installation

Add the package to your `go.mod` file or workspace:

```go
require github.com/nativebpm/connectors/cryptenv v0.0.0
```

---

## Usage

### 1. Initialization

Initialize a new secure environment using a **Master Key** (this key is hashed using SHA-256 to derive the 32-byte AES key):

```go
se, err := cryptenv.NewSecureEnv("my-master-password")
if err != nil {
    log.Fatalf("Failed to initialize secure env: %v", err)
}
```

### 2. Loading age Encrypted Secrets

#### Symmetric Decryption (Passphrase-based)
Decrypt and parse a `.env.age` file encrypted symmetrically with `age`:

```go
err = se.LoadAgeSymmetric("path/to/secrets.env.age", "my-master-password")
```

#### Asymmetric Decryption (Key-based)
Decrypt and parse an `.env.age` file encrypted asymmetrically (e.g., using a server's identity key or standard SSH private key):

```go
identity, err := age.ParseX25519Identity("AGE-SECRET-KEY-...")
err = se.LoadAgeAsymmetric("path/to/secrets.env.age", identity)
```

### 3. Reading Secrets

#### Basic Read
Retrieve and decrypt a secret value on the fly:

```go
dbPass, err := se.Get("DB_PASSWORD")
if err != nil {
    log.Fatalf("Secret not found: %v", err)
}
```

#### Read with Default Fallback
```go
logLevel := se.GetDefault("LOG_LEVEL", "info")
```

---

## Struct Unmarshaling (Fluent / Declarative Config)

Populate a Go struct using `env` and `envDefault` tags directly:

```go
type Config struct {
    DBUser   string `env:"DB_USER"`
    DBPass   string `env:"DB_PASSWORD"`
    DBPort   int    `env:"DB_PORT" envDefault:"5432"`
    SSLMode  bool   `env:"SSL_MODE"`
}

var cfg Config
err = se.Unmarshal(&cfg)
```

---

## YAML Configuration Secrets

If you store structured YAML blocks inside your secrets, you can unmarshal them directly into structs:

```go
type DatabaseConfig struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}

var dbCfg DatabaseConfig
err = se.GetYAML("DB_CONFIG_YAML", &dbCfg)
```

---

## Third-Party Integrations (e.g., `caarlos0/env`)

You can convert the secure environment variables to a `map[string]string` to integrate with libraries like `caarlos0/env`:

```go
import "github.com/caarlos0/env/v11"

envMap, err := se.ToMap()
if err == nil {
    env.ParseWithOptions(&cfg, env.Options{Environment: envMap})
}
```

---

## License

MIT - See [LICENSE](../LICENSE)
