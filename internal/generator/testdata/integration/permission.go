package integration

type permission int

const (
	permissionNone      permission = iota // enum:alias=n,none
	permissionRead                        // enum:alias=r
	permissionWrite                       // enum:alias=w
	permissionReadWrite                   // enum:alias=rw,read-write
)
