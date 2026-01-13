package types

// GoogleUser - User info from Google OAuth
type GoogleUser struct {
	ID            string `json:"sub"`            // Unique Google ID
	Email         string `json:"email"`          // User's email address
	EmailVerified bool   `json:"email_verified"` // Whether email is verified
	Name          string `json:"name"`           // Full name
	GivenName     string `json:"given_name"`     // First name
	FamilyName    string `json:"family_name"`    // Last name
	Picture       string `json:"picture"`        // Profile picture URL
	Locale        string `json:"locale"`         // Language/locale
	Hd            string `json:"hd,omitempty"`   // Hosted domain (for G Suite)
}
