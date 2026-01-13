package types

type GithubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type OAuthUserInfo struct {
	ID            int    `json:"id"`
	Login         string `json:"login"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	AvatarURL     string `json:"avatar_url"`
	EmailVerified bool   `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
	Hd            string `json:"hd,omitempty"`
}
