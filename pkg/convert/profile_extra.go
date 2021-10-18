package convert

// These functions are kept separately as profile.pb.go is a generated file

import (
	"fmt"
	"log"
	"sort"

	"github.com/valyala/bytebufferpool"
)

func (x *Profile) Get(sampleType string, cb func(name []byte, val int)) error {
	valueIndex := 0
	if sampleType != "" {
		for i, v := range x.SampleType {
			if x.StringTable[v.Type] == sampleType {
				valueIndex = i
				break
			}
		}
	}

	b := bytebufferpool.Get()
	defer bytebufferpool.Put(b)

	log.Println("get ---")

	for _, s := range x.Sample {
		for i := len(s.LocationId) - 1; i >= 0; i-- {
			name, ok := x.findFunctionName(s.LocationId[i])
			if !ok {
				continue
			}
			if b.Len() > 0 {
				_ = b.WriteByte(';')
			}
			_, _ = b.WriteString(name)
		}
		cb(b.Bytes(), int(s.Value[valueIndex]))

		for _, l := range s.Label {
			if l.Str != 0 {
				fmt.Printf("label %s\n", b.Bytes())
				fmt.Printf("tags %s %s\n", x.StringTable[l.Key], x.StringTable[l.Str])
			}
		}
		b.Reset()
	}

	return nil
}

func (x *Profile) findFunctionName(locID uint64) (string, bool) {
	if loc, ok := x.findLocation(locID); ok {
		if fn, ok := x.findFunction(loc.Line[0].FunctionId); ok {
			return x.StringTable[fn.Name], true
		}
	}
	return "", false
}

func (x *Profile) findTag(keyID uint64) (string, string, bool) {
	// if loc, ok := x.findLocation(locID); ok {
	// 	if fn, ok := x.findFunction(loc.Line[0].FunctionId); ok {
	// 		return x.StringTable[fn.Name], true
	// 	}
	// }
	return "", "", false
}

func (x *Profile) findLocation(lid uint64) (*Location, bool) {
	idx := sort.Search(len(x.Location), func(i int) bool {
		return x.Location[i].Id >= lid
	})
	if idx < len(x.Location) {
		if l := x.Location[idx]; l.Id == lid {
			return l, true
		}
	}
	return nil, false
}

func (x *Profile) findFunction(fid uint64) (*Function, bool) {
	idx := sort.Search(len(x.Function), func(i int) bool {
		return x.Function[i].Id >= fid
	})
	if idx < len(x.Function) {
		if f := x.Function[idx]; f.Id == fid {
			return f, true
		}
	}
	return nil, false
}
