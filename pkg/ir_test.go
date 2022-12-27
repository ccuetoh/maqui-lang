package maqui

import (
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValueLookup(t *testing.T) {
	vals := NewValueLookup()

	val1 := constant.NewInt(types.I32, 1)
	val2 := constant.NewInt(types.I32, 2)

	vals.Set("id1", val1)
	vals.Set("id2", val2)

	assert.Equal(t, val1, vals.Get("id1"))
	assert.Equal(t, val2, vals.Get("id2"))
}

func TestValueLookupInherit(t *testing.T) {
	vals1 := NewValueLookup()

	val1 := constant.NewInt(types.I32, 1)
	val2 := constant.NewInt(types.I32, 2)

	vals1.Set("id1", val1)
	vals1.Set("id2", val2)

	vals2 := NewValueLookup()

	val3 := constant.NewInt(types.I32, 3)
	val4 := constant.NewInt(types.I32, 4)

	vals2.Set("id1", val3)
	vals2.Set("id4", val4)

	vals1.Inherit(vals2)

	assert.Equal(t, val3, vals1.Get("id1"))
	assert.Equal(t, val2, vals1.Get("id2"))
	assert.Equal(t, val4, vals1.Get("id4"))
}
