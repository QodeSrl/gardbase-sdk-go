module github.com/qodesrl/gardbase-sdk-go

go 1.24.4

require (
	github.com/qodesrl/gardbase/pkg/api v0.1.0
	github.com/qodesrl/gardbase/pkg/crypto v0.1.0
	github.com/qodesrl/gardbase/pkg/enclaveproto v0.1.0
	golang.org/x/crypto v0.47.0
)

require (
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/sys v0.40.0 // indirect
)

replace github.com/qodesrl/gardbase/pkg/api => ../project/pkg/api

replace github.com/qodesrl/gardbase/pkg/crypto => ../project/pkg/crypto

replace github.com/qodesrl/gardbase/pkg/enclaveproto => ../project/pkg/enclaveproto
