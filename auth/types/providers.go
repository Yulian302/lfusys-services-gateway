package types

type ProviderName int

const (
	GithubProvider = iota
	GoogleProvider
)

var Providers = map[ProviderName]string{
	GithubProvider: "github",
	GoogleProvider: "google",
}

func (prov ProviderName) String() string {
	return Providers[prov]
}
