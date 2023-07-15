//go:build !go1.21

package base64dq

import "sort"

func (dm *decodeMap) search(r rune) byte {
	i := sort.Search(dm.Len(), func(i int) bool { return dm.runes[i] >= r })
	if i < dm.Len() && dm.runes[i] == r {
		return dm.bytes[i]
	}
	return 0xff
}
