package wireless

// Bind provides interface type binding for the type 'to' to the interface type 'iface'.
// Example:
// 	wireless.Bind(new(io.Reader), new(*bytes.Reader))
func Bind(iface interface{}, to interface{}) Provider {
	return &bindingProvider{iface: iface, to: to}
}

// Value is the direct value provider type. This function is used to provide the
func Value(value interface{}) Provider {
	return &valueProvider{v: value}
}

// InterfaceValue defines interface value casting that could be done for proper injection.
// Example:
//	wireless.InterfaceValue(new(io.Reader), new(*bytes.Reader))
func InterfaceValue(iface interface{}, to interface{}) Provider {
	return &interfaceValueProvider{iface: iface, value: to}
}

// NewSet creates a new ProviderSet.
func NewSet(providers ...Provider) ProviderSet {
	return providers
}

// Func declares a provider function that creates and optionally cleans a new value.
func Func(in interface{}) Provider {
	return &funcProvider{v: in}
}

// IfNotExists sets up input provider in the injector only no provider is defined for given type.
func IfNotExists(p Provider) Provider {
	p.setOptions(func(o *providerOptions) { o.ifNotExists = true })
	return p
}

// Namespace sets up provider namespace.
func Namespace(namespace string, p Provider) Provider {
	p.setOptions(func(o *providerOptions) { o.namespace = namespace })
	return p
}

type providerOption func(o *providerOptions)

type providerOptions struct {
	ifNotExists bool
	namespace   string
}

// Provider is the interface that defines a provider.
type Provider interface {
	setOptions(o ...providerOption)
}

// ProviderSet is the wireless injection provider set.
type ProviderSet []Provider

func (ps ProviderSet) setOptions(op ...providerOption) {
	for _, p := range ps {
		p.setOptions(op...)
	}
}

// bindingProvider is the injection binding of interface to some value.
type bindingProvider struct {
	iface interface{}
	to    interface{}
	providerOptions
}

func (b *bindingProvider) setOptions(options ...providerOption) {
	for _, os := range options {
		os(&b.providerOptions)
	}
}

type interfaceValueProvider struct {
	iface interface{}
	value interface{}
	providerOptions
}

func (i *interfaceValueProvider) setOptions(options ...providerOption) {
	for _, os := range options {
		os(&i.providerOptions)
	}
}

type valueProvider struct {
	v interface{}
	providerOptions
}

func (v *valueProvider) setOptions(options ...providerOption) {
	for _, os := range options {
		os(&v.providerOptions)
	}
}

// funcProvider is the provider function used by the
type funcProvider struct {
	v interface{}
	providerOptions
}

func (f *funcProvider) setOptions(options ...providerOption) {
	for _, os := range options {
		os(&f.providerOptions)
	}
}
