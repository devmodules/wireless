package wireless

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err)

		var ptr *testType
		err = i.InjectAs(&ptr)
		require.NoError(t, err)

		assert.Equal(t, ptr, provider)
	})

	t.Run("Type", func(t *testing.T) {
		i := New()

		provider := testType{v: "tt"}

		i.Provide(
			Value(provider),
		)

		err := i.Resolve()
		require.NoError(t, err)

		var as testType
		err = i.InjectAs(&as)
		require.NoError(t, err)

		assert.Equal(t, as, provider)
	})

	t.Run("Simple", func(t *testing.T) {
		var called bool
		taken := "taken"
		newType := func() (testType, func(), error) {
			return testType{v: taken}, func() { called = true }, nil
		}

		i := New()
		i.Provide(
			Func(newType),
		)
		err := i.Resolve()
		require.NoError(t, err)

		var tt testType
		err = i.InjectAs(&tt)
		require.NoError(t, err)

		assert.Equal(t, tt.v, taken)

		i.Clean()

		assert.True(t, called)
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
		// Expected flow is *testType value -> newType -> binding
		i.Provide(
			Func(newB),
			Bind(new(interfaceType), new(testType)),
			Value(ptrValue),
			Func(newType),
		)
		err := i.Resolve()
		require.NoError(t, err)

		var it interfaceType
		err = i.InjectAs(&it)
		require.NoError(t, err)

		tt, ok := it.(testType)
		require.True(t, ok)

		assert.Equal(t, tt.v, taken)

		i.Clean()
		assert.True(t, !bts.IsZero())
		assert.True(t, !tpTs.IsZero())
		assert.True(t, tpTs.Before(bts))
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
		assert.Error(t, err)
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
		require.NoError(t, err)

		var dv d
		err = i.Inject(&dv)
		require.NoError(t, err)

		assert.True(t, dv.C.started)
		assert.True(t, dv.B.started)
		assert.True(t, dv.A.started)
	})
}
