package wireless

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

var (
	errorType   = reflect.TypeOf(new(error)).Elem()
	cleanupFunc = reflect.FuncOf(nil, nil, false)
)

// Error definitions returned by the injector.
var (
	ErrAlreadyResolved = errors.New("injector already resolved")
	ErrNotResolved     = errors.New("injector not resolved")
	ErrAlreadyCleaned  = errors.New("injector already cleaned")
)

// New creates a new injector.
func New() *Injector {
	i := &Injector{
		values:       map[reflect.Type]reflect.Value{},
		providersMap: map[reflect.Type]*providerFunc{},
		bindings:     map[reflect.Type]reflect.Type{},
	}
	i.values[reflect.TypeOf(i)] = reflect.ValueOf(i)
	return i
}

// Injector is dynamic connection provider.
type Injector struct {
	id            int64
	lock          sync.RWMutex
	resolved      bool
	values        map[reflect.Type]reflect.Value
	providersMap  map[reflect.Type]*providerFunc
	providerFuncs []*providerFunc
	bindings      map[reflect.Type]reflect.Type

	valueProviders          []*valueProvider
	bindingProviders        []*bindingProvider
	funcProviders           []*funcProvider
	interfaceValueProviders []*interfaceValueProvider

	errors  multiError
	cleaned bool
}

// Inject tries to inject all the fields within provided input pointer to struct.
// In order to omit a field it might use a struct field tag: 'wireless:"-"'.
// Example:
//
//	type ExampleType struct {
//		InjectMe 	*OtherType
//		SkipMe 		*DifferentType `wireless:"-"
//		skipPrivate *PrivateType
//	}
func (i *Injector) Inject(in interface{}) error {
	i.lock.RLock()
	defer i.lock.RUnlock()
	if !i.resolved {
		return ErrNotResolved
	}
	if i.cleaned {
		return ErrAlreadyCleaned
	}
	if len(i.errors) > 0 {
		return i.errors
	}
	if in == nil {
		return errors.New("input injection type is nil")
	}
	rv := reflect.ValueOf(in)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Type().Kind() != reflect.Struct {
		return fmt.Errorf("input injection type is not a pointer to the struct but: %T", in)
	}
	for j := 0; j < rv.NumField(); j++ {
		fv := rv.Field(j)
		ft := rv.Type().Field(j)
		if !ft.IsExported() {
			continue
		}
		if tv := ft.Tag.Get("wireless"); tv == "-" {
			continue
		}
		fv = fv.Addr()
		if err := i.injectAs(fv); err != nil {
			return err
		}
	}
	// Sort the providers again to have the least dependent be on the end.
	sort.Slice(i.providerFuncs, func(j, k int) bool {
		return i.providerFuncs[j].depth < i.providerFuncs[k].depth
	})
	return nil
}

// InjectAs gets the injector for the input pointer to type.
func (i *Injector) InjectAs(as interface{}) error {
	i.lock.RLock()
	defer i.lock.RUnlock()

	if !i.resolved {
		return ErrNotResolved
	}
	if i.cleaned {
		return ErrAlreadyCleaned
	}
	if len(i.errors) > 0 {
		return i.errors
	}
	if as == nil {
		return errors.New("input injection type is nil")
	}

	rVal := reflect.ValueOf(as)
	if rVal.Kind() != reflect.Ptr {
		return errors.New("input injection type is not a pointer")
	}
	err := i.injectAs(rVal)
	if err != nil {
		return err
	}

	// Sort the providers again to have the least dependent be on the end.
	sort.Slice(i.providerFuncs, func(j, k int) bool {
		return i.providerFuncs[j].depth < i.providerFuncs[k].depth
	})
	return nil
}

func (i *Injector) injectAs(rVal reflect.Value) error {
	elem := rVal.Type().Elem()
	provider, ok := i.values[elem]
	if ok {
		rVal.Elem().Set(provider)
		return nil
	}
	pf, ok := i.providersMap[elem]
	if !ok {
		bv, ok := i.bindings[elem]
		if !ok {
			return fmt.Errorf("injector not found for the type: %s", elem)
		}
		provider, ok = i.values[bv]
		if ok {
			rVal.Elem().Set(provider)
			return nil
		}
		pf, ok = i.providersMap[bv]
		if !ok {
			return fmt.Errorf("injector not found for the type: %s", elem)
		}
	}
	// Check if the value of the provider set is already resolved.
	if pf.outValue.IsValid() {
		rVal.Elem().Set(pf.outValue)
		return nil
	}

	err := i.executeNecessaryProviders(pf)
	if err != nil {
		return err
	}
	rVal.Elem().Set(pf.outValue)
	return nil
}

func (i *Injector) executeNecessaryProviders(pf *providerFunc) error {
	providers := pf.getProviders()
	for _, p := range providers {
		if p.outValue.IsValid() {
			continue
		}
		ins := make([]reflect.Value, len(p.in))
		for j, in := range p.in {
			switch it := in.(type) {
			case reflect.Value:
				ins[j] = it
			case boundProviderFunc:
				ins[j] = it.f.outValue
			case *providerFunc:
				ins[j] = it.outValue
			}
		}
		outs := p.value.Call(ins)
		if p.errOut > 0 {
			if errVal := outs[p.errOut]; !errVal.IsNil() {
				err := errVal.Interface().(error)
				return err
			}
		}
		if p.cleanupOut > 0 {
			cf := outs[p.cleanupOut]
			if !cf.IsNil() {
				p.cleanup = cf
			}
		}
		p.outValue = outs[0]
		i.providerFuncs = append(i.providerFuncs, p)
	}
	return nil
}

// Provide builds up provider injector.
func (i *Injector) Provide(providers ...Provider) {
	for _, provider := range providers {
		i.addProviders(provider)
	}
}

func (i *Injector) addProviders(providers ...Provider) {
	for _, provider := range providers {
		switch pt := provider.(type) {
		case *interfaceValueProvider:
			i.interfaceValueProviders = append(i.interfaceValueProviders, pt)
		case *bindingProvider:
			i.bindingProviders = append(i.bindingProviders, pt)
		case *funcProvider:
			i.funcProviders = append(i.funcProviders, pt)
		case *valueProvider:
			i.valueProviders = append(i.valueProviders, pt)
		case ProviderSet:
			i.addProviders(pt...)
		}
	}
}

// Resolve the injection providers.
func (i *Injector) Resolve() error {
	if i.cleaned {
		return ErrAlreadyCleaned
	}
	if i.resolved {
		return ErrAlreadyResolved
	}
	if len(i.errors) > 0 {
		return i.errors
	}
	i.lock.Lock()
	defer i.lock.Unlock()

	i.resolveBindings()
	i.resolveInterfaceValues()
	i.resolveValues()
	if err := i.resolveProvideFunctions(); err != nil {
		return err
	}

	i.resolved = true
	return nil
}

// Clean execute all clean functions of the provider functions in reverse order to which it was called.
func (i *Injector) Clean() {
	if i.cleaned {
		return
	}
	i.lock.Lock()
	defer i.lock.Unlock()
	for j := len(i.providerFuncs) - 1; j >= 0; j-- {
		provider := i.providerFuncs[j]
		if !provider.cleanup.IsValid() {
			continue
		}
		provider.cleanup.Call(nil)
	}
	i.cleaned = true
}

// Value sets up raw value that could be used as an injection for other types.
func (i *Injector) resolveValues() {
	if len(i.errors) > 0 {
		return
	}
	for _, vp := range i.valueProviders {
		if vp.v == nil {
			i.errors = append(i.errors, errors.New("input value provider is nil"))
			return
		}

		rv := reflect.ValueOf(vp.v)
		_, ok := i.values[rv.Type()]
		if ok {
			i.errors = append(i.errors, fmt.Errorf("provider for type: %s already exists", rv.Type().String()))
			continue
		}
		i.values[rv.Type()] = rv
	}
}

func (i *Injector) resolveInterfaceValues() {
	if len(i.errors) > 0 {
		return
	}
	for _, vp := range i.interfaceValueProviders {
		if vp.value == nil {
			i.errors = append(i.errors, errors.New("input value provider is nil"))
			return
		}
		to := reflect.ValueOf(vp.value)
		it := reflect.TypeOf(vp.iface)
		if it.Elem().Kind() != reflect.Interface {
			i.errors = append(i.errors, fmt.Errorf("one of provided interface values are not using interface as type: %s -> %s", it.String(), to.String()))
			continue
		}
		if !to.CanConvert(it) {
			i.errors = append(i.errors, fmt.Errorf("one of provided interface values type does not implement interface type: %s -> %s", it.String(), to.String()))
			continue
		}

		_, ok := i.values[it]
		if ok {
			i.errors = append(i.errors, fmt.Errorf("provider for type: %s already exists", to.Type().String()))
			continue
		}
		i.values[it] = to.Convert(it)
	}
}

// Provide registers new provider injector functions.
func (i *Injector) resolveProvideFunctions() error {
	i.matchProviderFuncs()
	if len(i.errors) > 0 {
		return i.errors
	}

	err := i.resolveProvidersDependencies()
	if err != nil {
		return err
	}

	providers := make([]*providerFunc, len(i.providersMap))
	for _, p := range i.providersMap {
		providers[p.id-1] = p
	}
	visited, dfsVisited := make([]bool, len(i.providersMap)), make([]bool, len(i.providersMap))
	for _, p := range providers {
		if !visited[p.id-1] {
			trace, hasCycles := checkCycles(p, visited, dfsVisited)
			if hasCycles {
				return fmt.Errorf("dependenc cycle detected %s", strings.Join(trace, "<-"))
			}
		}
	}
	return nil
}

func checkCycles(p *providerFunc, visited []bool, dfsVisited []bool) ([]string, bool) {
	visited[p.id-1] = true
	dfsVisited[p.id-1] = true
	max := -1
	for _, dep := range p.dependencies {
		if !visited[dep.id-1] {
			trace, hasCycle := checkCycles(dep, visited, dfsVisited)
			if hasCycle {
				return append(trace, p.out.String()), true
			}
		} else if dfsVisited[dep.id-1] {
			return []string{dep.out.String()}, true
		}
		max = maxInt(max, dep.depth)
	}
	p.depth = max + 1
	dfsVisited[p.id-1] = false
	return nil, false
}

func (i *Injector) resolveProvidersDependencies() error {
	for _, p := range i.providersMap {
		p.in = make([]interface{}, len(p.inTypes))
		for j, in := range p.inTypes {
			vt, ok := i.values[in]
			if ok {
				p.in[j] = vt
				continue
			}

			pf, ok := i.providersMap[in]
			if ok {
				p.in[j] = pf
				p.dependencies = append(p.dependencies, pf)
				continue
			}

			// Check if the input is an interface bound to some other type.
			bt, ok := i.bindings[in]
			if ok {
				// Check if the bound interface is a registered value.
				vt, ok = i.values[bt]
				if ok {
					p.in[j] = vt.Convert(in)
					continue
				}

				// Check if the bound interface is a result of the provider function.
				pf, ok = i.providersMap[bt]
				if ok {
					p.in[j] = boundProviderFunc{f: pf, boundAs: in}
					p.dependencies = append(p.dependencies, pf)
					continue
				}
			}

			return fmt.Errorf("no provider found for the %s type", in.String())
		}
		p.depth = -1
	}
	return nil
}

func (i *Injector) matchProviderFuncs() {
	for _, fp := range i.funcProviders {
		rv := reflect.ValueOf(fp.v)
		if rv.Kind() != reflect.Func {
			i.errors = append(i.errors, fmt.Errorf("provider %T is not a function ", fp.v))
			continue
		}
		rvt := rv.Type()
		pf := providerFunc{id: i.nextID(), value: rv, errOut: -1, cleanupOut: -1}

		numDependencies := rv.Type().NumIn()
		for j := 0; j < numDependencies; j++ {
			pf.inTypes = append(pf.inTypes, rvt.In(j))
		}

		numOut := rvt.NumOut()
		switch numOut {
		case 1:
			// Only provided type.
			pf.out = rvt.Out(0)
		case 2:
			// Provided type and error or provided type and cleanup func.
			pf.out = rvt.Out(0)
			second := rvt.Out(1)
			switch {
			case second.AssignableTo(errorType):
				pf.errOut = 1
			case second.AssignableTo(cleanupFunc):
				pf.cleanupOut = 1
			default:
				i.errors = append(i.errors, fmt.Errorf("provider: %T has invalid out second variable type %s", fp.v, second))
				continue
			}
		case 3:
			// Provided type error and cleanup type.
			pf.out = rvt.Out(0)
			// Provided type and error or provided type and cleanup func.
			pf.cleanupOut = 1
			if !rvt.Out(1).AssignableTo(cleanupFunc) {
				i.errors = append(i.errors, fmt.Errorf("provider: %T has invalid out second variable type expected to be a cancel function but is: %s", fp.v, rvt.Out(1)))
				pf.cleanupOut = 0
				continue
			}

			pf.errOut = 2
			if !rvt.Out(2).AssignableTo(errorType) {
				i.errors = append(i.errors, fmt.Errorf("provider: %T has invalid out second variable type expected to be an error but is: %s", fp.v, rvt.Out(1)))
				pf.errOut = 0
				continue
			}
		default:
			i.errors = append(i.errors, fmt.Errorf("provider: %T have invalid returned variables number", fp.v))
			continue
		}
		_, ok := i.providersMap[pf.out]
		if ok {
			if fp.ifNotExists {
				continue
			}
			i.errors = append(i.errors, fmt.Errorf("provider already registered for type: %s", pf.out.String()))
			continue
		}
		i.providersMap[pf.out] = &pf
	}
}

func (i *Injector) resolveBindings() {
	for _, binding := range i.bindingProviders {
		it := reflect.TypeOf(binding.iface)
		to := reflect.TypeOf(binding.to)
		if it.Kind() != reflect.Ptr || to.Kind() != reflect.Ptr {
			i.errors = append(i.errors, fmt.Errorf("one of provided bindings are not defining values with `new` statement: %T -> %T", binding.iface, binding.to))
			continue
		}
		it = it.Elem()
		to = to.Elem()
		if it.Kind() != reflect.Interface {
			i.errors = append(i.errors, fmt.Errorf("one of provided bindings are not using interface as type: %s -> %s", it.String(), to.String()))
			continue
		}
		if !to.Implements(it) {
			i.errors = append(i.errors, fmt.Errorf("one of provided bindings type does not implement interface type: %s -> %s", it.String(), to.String()))
			continue
		}

		_, ok := i.bindings[it]
		if ok {
			if binding.ifNotExists {
				continue
			}
			i.errors = append(i.errors, fmt.Errorf("binding between: %s and %s is already defined", it, to))
			continue
		}
		i.bindings[it] = to
	}
}

func (i *Injector) nextID() int64 {
	i.id++
	return i.id
}

type providerFunc struct {
	id           int64
	value        reflect.Value
	inTypes      []reflect.Type
	in           []interface{}
	dependencies []*providerFunc
	out          reflect.Type
	errOut       int
	cleanupOut   int
	outValue     reflect.Value
	cleanup      reflect.Value
	depth        int
}

func (p *providerFunc) getProviders() []*providerFunc {
	var providers []*providerFunc
	for _, in := range p.dependencies {
		providers = append(providers, in.getProviders()...)
	}
	providers = append(providers, p)
	return providers
}

type boundProviderFunc struct {
	f       *providerFunc
	boundAs reflect.Type
}

func maxInt(i, j int) int {
	if i > j {
		return i
	}
	return j
}

type multiError []error

func (m multiError) Error() string {
	sb := strings.Builder{}
	for i, e := range m {
		sb.WriteString(e.Error())
		if i != len(m)-1 {
			sb.WriteRune(';')
		}
	}
	return sb.String()
}
