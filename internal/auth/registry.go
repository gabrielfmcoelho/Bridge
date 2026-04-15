package auth

// ProviderRegistry manages registered authentication providers.
type ProviderRegistry struct {
	providers map[string]AuthProvider
}

// NewProviderRegistry creates an empty provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]AuthProvider),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(p AuthProvider) {
	r.providers[p.Name()] = p
}

// Get returns a provider by name.
func (r *ProviderRegistry) Get(name string) (AuthProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// EnabledProviders returns all currently enabled providers.
func (r *ProviderRegistry) EnabledProviders() []AuthProvider {
	var result []AuthProvider
	for _, p := range r.providers {
		if p.Enabled() {
			result = append(result, p)
		}
	}
	return result
}

// DirectLoginProviders returns enabled providers that support username+password.
func (r *ProviderRegistry) DirectLoginProviders() []AuthProvider {
	var result []AuthProvider
	for _, p := range r.EnabledProviders() {
		if p.SupportsDirectLogin() {
			result = append(result, p)
		}
	}
	return result
}

// OAuthProviders returns enabled providers that use OAuth/OIDC redirect flows.
func (r *ProviderRegistry) OAuthProviders() []AuthProvider {
	var result []AuthProvider
	for _, p := range r.EnabledProviders() {
		if !p.SupportsDirectLogin() {
			result = append(result, p)
		}
	}
	return result
}
