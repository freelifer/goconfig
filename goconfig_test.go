package goconfig

import (
	"testing"
)

func Test_Goconfig(t *testing.T) {
	xxx := MustValue("", "xxx", "")
	t.Log(xxx)
	name := MustValue("app", "name", "default")
	t.Log(name)

	// i_a=1
	// f_b=1.2
	// b_c=false
	// l_d=1
	i_a := MustInt("test", "i_a", 0)
	f_b := MustFloat64("test", "f_b", 0.0)
	b_c := MustBool("test", "b_c", true)
	l_d := MustInt64("test", "l_d", 1)
	t.Log(i_a)
	t.Log(f_b)
	t.Log(b_c)
	t.Log(l_d)
}
