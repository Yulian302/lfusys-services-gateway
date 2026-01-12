package types

type ProviderName int

const (
	GithubProvider = iota
)

var Providers = map[ProviderName]string{
	GithubProvider: "github",
}

func (prov ProviderName) String() string {
	return Providers[prov]
}
