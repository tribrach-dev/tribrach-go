package drifty // want "TRG0205 the manifest links Shop.gone to example.com/drifty.Gone" "TRG0205 the manifest links Shop.total to example.com/drifty.Total"

type Cart struct{ items []int }

//tribrach:link Shop.add
//tribrach:stamp 0123456789abcdef
func Add(xs []int, x int) []int { return append(xs, x) } // want "TRG0201 Shop.add changed in Lean"

//tribrach:link Shop.remove
func Remove(xs []int) []int { return xs[:0] } // want "TRG0202 Remove links Shop.remove but records no stamp"

//tribrach:link Shop.sum
//tribrach:stamp 0123456789abcdef
func Sum(xs []int) int { return len(xs) } // want "TRG0203 the manifest .tribrach.manifest.json. has no Go link for Shop.sum"

//tribrach:link Shop.total
//tribrach:stamp 0123456789abcdef
func TotalV2(xs []int) int { return len(xs) } // want "TRG0204 the manifest links Shop.total to example.com/drifty.Total, not example.com/drifty.TotalV2"

//tribrach:link Shop.clear
//tribrach:stamp 0123456789abcdef
func (c *Cart) Clear() { c.items = nil }

//tribrach:stamp 0123456789abcdef
func Orphan() {} // want "TRG0104 Orphan has a //tribrach:stamp but no //tribrach:link"
