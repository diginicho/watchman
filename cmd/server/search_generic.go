// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sync"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
)

type Result[T any] struct {
	Data T

	match           float64
	algo            string
	precomputedName string
	precomputedAlts []string
}

func (e Result[T]) MarshalJSON() ([]byte, error) {
	// Due to a problem with embedding type parameters we have to dig into
	// the parameterized type fields and include them in one object.
	//
	// Helpful Tips:
	// https://stackoverflow.com/a/64420452
	// https://github.com/golang/go/issues/41563

	v := reflect.ValueOf(e.Data)

	result := make(map[string]interface{})
	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i)
		value := v.Field(i)

		if key.IsExported() {
			result[key.Name] = value.Interface()
		}
	}

	result["match"] = e.match

	return json.Marshal(result)
}

func calculateBestWeight(s1, s2 string) (float64, string) {
	var weight float64
	var algo string

	if s1 == s2 {
		weight = 1
		algo = "direct"
		fmt.Println("direct match found, ding ding ding")
		return weight, algo
	}
	jwWeight := jaroWinkler(s1, s2)
	dlWeight := strutil.Similarity(s1, s2, metrics.NewLevenshtein())
	hamWeight := strutil.Similarity(s1, s2, metrics.NewHamming())

	if jwWeight > dlWeight && jwWeight > hamWeight {
		weight = jwWeight
		algo = "Jaro-Winkler"
	}
	if dlWeight > jwWeight && dlWeight > hamWeight {
		weight = dlWeight
		algo = "Levenshtein"
	}
	if hamWeight > dlWeight && hamWeight > jwWeight {
		weight = hamWeight
		algo = "Hamming"
	}

	fmt.Printf("J/W: %v, D/L: %v, HAM: %v\n", jwWeight, dlWeight, hamWeight)
	fmt.Println(algo, weight)
	return weight, algo
}

func topResults[T any](limit int, minMatch float64, name string, data []*Result[T]) []*Result[T] {
	if len(data) == 0 {
		return nil
	}

	name = precompute(name)
	xs := newLargest(limit, minMatch)

	var wg sync.WaitGroup
	wg.Add(len(data))

	for i := range data {
		go func(i int) {
			defer wg.Done()
			weight, algo := calculateBestWeight(data[i].precomputedName, name)

			it := &item{
				value:  data[i],
				weight: weight,
				algo:   algo,
			}

			for _, alt := range data[i].precomputedAlts {
				if alt == "" {
					continue
				}
				it.weight = math.Max(it.weight, jaroWinkler(alt, name))
			}

			xs.add(it)
		}(i)
	}
	wg.Wait()

	out := make([]*Result[T], 0)
	for _, thisItem := range xs.items {
		if v := thisItem; v != nil {
			vv, ok := v.value.(*Result[T])
			if !ok {
				continue
			}
			res := &Result[T]{
				Data:            vv.Data,
				match:           vv.match,
				algo:            vv.algo,
				precomputedName: vv.precomputedName,
				precomputedAlts: vv.precomputedAlts,
			}
			out = append(out, res)
		}
	}
	return out
}
