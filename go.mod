module reverse-challenge-system

go 1.24

toolchain go1.24.7

require (
	github.com/fardream/go-bcs v0.8.7
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/joho/godotenv v1.5.1
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/pattonkan/sui-go v0.1.8
	github.com/rs/zerolog v1.32.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.0
)

require (
	github.com/Khan/genqlient v0.8.1 // indirect
	github.com/btcsuite/btcd/btcutil v1.1.6 // indirect
	github.com/coder/websocket v1.8.13 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/luxfi/go-bip39 v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/tyler-smith/go-bip39 v1.1.0 // indirect
	github.com/vektah/gqlparser/v2 v2.5.19 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
)

replace github.com/tyler-smith/go-bip39 => github.com/luxfi/go-bip39 v1.1.0
