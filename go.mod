module github.com/kubewarden/safe-labels-policy

go 1.20

replace github.com/go-openapi/strfmt => github.com/kubewarden/strfmt v0.1.3

require (
	github.com/deckarep/golang-set/v2 v2.5.0
	github.com/kubewarden/gjson v1.7.2
	github.com/kubewarden/policy-sdk-go v0.5.1
	github.com/wapc/wapc-guest-tinygo v0.3.3
)

require (
	github.com/go-openapi/strfmt v0.21.3 // indirect
	github.com/kubewarden/k8s-objects v1.27.0-kw4 // indirect
	github.com/tidwall/match v1.0.3 // indirect
	github.com/tidwall/pretty v1.0.2 // indirect
)
