//go:build go1.21

package base64dq

import "slices"

func (dm *decodeMap) search(r rune) byte {
	i, ok := slices.BinarySearch(dm.runes[:], r)
	if !ok {
		return 0xff
	}
	return dm.bytes[i]
}
