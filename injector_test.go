package wireless

import (
	"testing"
	"time"
)

type interfaceType interface {
	isInterfacer()
}

type testType struct {
	v string
}

func (t testType) isInterfacer() {}

func TestInjector(t *testing.T) {
	t.Run("Pointer", func(t *testing.T) {
		i := New()

		provider := &testType{v: "ptr"}
		i.Provide(
			Value(provider),
		)
		err := i.Resolve()
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		var ptr *testType
		err = i.InjectAs(&ptr)
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		if ptr != provider {
			t.Errorf("Expected %v, got %v", provider, ptr)
		}
	})

	t.Run("Type", func(t *testing.T) {
		i := New()

		provider := testType{v: "tt"}

		i.Provide(
			Value(provider),
		)

		err := i.Resolve()
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		var as testType
		err = i.InjectAs(&as)
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		if as != provider {
			t.Errorf("Expected %v, got %v", provider, as)
		}
	})

	t.Run("Simple", func(t *testing.T) {
		var called bool
		taken := "taken"
		newType := func(i *Injector) (testType, func(), error) {
			return testType{v: taken}, func() { called = true }, nil
		}

		i := New()
		i.Provide(
			Func(newType),
		)
		err := i.Resolve()
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		var tt testType
		err = i.InjectAs(&tt)
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		if tt.v != taken {
			t.Errorf("Expected %v, got %v", taken, tt.v)
		}

		i.Clean()

		if !called {
			t.Error("Expected true, got false")
		}
	})

	t.Run("Deps", func(t *testing.T) {
		var (
			bts  time.Time
			tpTs time.Time
		)
		taken := "taken"
		type b struct {
			ptr *testType
		}

		newB := func(ptr *testType) (b, func()) {
			return b{ptr: ptr}, func() { bts = time.Now() }
		}

		newType := func(bv b) (testType, func(), error) {
			return *bv.ptr, func() { tpTs = time.Now() }, nil
		}

		ptrValue := &testType{v: taken}
		i := New()
		i.Provide(
			Func(newB),
			Bind(new(interfaceType), new(testType)),
			Value(ptrValue),
			Func(newType),
		)
		err := i.Resolve()
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		var it interfaceType
		err = i.InjectAs(&it)
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		tt, ok := it.(testType)
		if !ok || tt.v != taken {
			t.Errorf("Expected %v, got %v", taken, tt.v)
		}

		i.Clean()
		if bts.IsZero() {
			t.Error("Expected non-zero time, got zero")
		}
		if tpTs.IsZero() || !tpTs.Before(bts) {
			t.Error("Expected tpTs before bts and non-zero, got zero or after bts")
		}
	})

	t.Run("Cycle", func(t *testing.T) {
		type a struct{}
		type b struct{}
		type c struct{}
		type d struct{}
		newA := func(in b) a { return a{} }
		newB := func(in c) b { return b{} }
		newC := func(in a, inD d) c { return c{} }

		i := New()
		i.Provide(
			Func(newA),
			Func(newB),
			Func(newC),
			Value(d{}),
		)
		err := i.Resolve()
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("Inject", func(t *testing.T) {
		type a struct{ started bool }
		type b struct{ started bool }
		type c struct{ started bool }
		type d struct {
			A a
			B b
			C c
		}
		newA := func(in b) a { return a{started: true} }
		newB := func(in c) b { return b{started: true} }
		newC := func() c { return c{started: true} }

		i := New()
		i.Provide(
			Func(newA),
			Func(newB),
			Func(newC),
		)
		err := i.Resolve()
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		var dv d
		err = i.Inject(&dv)
		if err != nil {
			t.Error("Expected no error, got", err)
		}

		if !dv.C.started || !dv.B.started || !dv.A.started {
			t.Errorf("Expected all true, got A: %t, B: %t, C: %t", dv.A.started, dv.B.started, dv.C.started)
		}
	})
}
