module github.com/nativebpm/connectors/cryptauth

go 1.26

require (
	filippo.io/age v1.1.1
	github.com/nativebpm/connectors/totp v0.0.1
	golang.org/x/crypto v0.31.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/nativebpm/connectors/totp => ../totp
