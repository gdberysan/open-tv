package domain

type Provider struct {
	ID          string
	Type        ProviderType
	BaseURL     string
	Credentials ProviderCredentials // interface tipada
	Priority    int
	IsActive    bool
}

type ProviderCredentials interface {
	credentialMarker() // método privado para type safety
}

type XtreamCredentials struct {
	Username string
	Password string
}

func (XtreamCredentials) credentialMarker() {}
