// Copyright 2021 TiKV Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// NOTE: The code in this file is based on code from the
// TiDB project, licensed under the Apache License v 2.0
//
// https://github.com/pingcap/tidb/tree/cc5e161ac06827589c4966674597c137cc9e809c/store/tikv/unionstore/memdb_test.go
//

// Copyright 2020 PingCAP, Inc.
//
// Copyright 2015 Wenbin Xiao
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package unionstore

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	leveldb "github.com/pingcap/goleveldb/leveldb/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tikverr "github.com/tikv/client-go/v2/error"
	"github.com/tikv/client-go/v2/kv"
)

type KeyFlags = kv.KeyFlags

func TestGetSet(t *testing.T) {
	testGetSet(t, newRbtDBWithContext())
	testGetSet(t, newArtDBWithContext())
}

func testGetSet(t *testing.T, db MemBuffer) {
	require := require.New(t)

	const cnt = 10000
	fillDB(db, cnt)

	var buf [4]byte
	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		v, err := db.Get(context.Background(), buf[:])
		require.Nil(err)
		require.Equal(v, buf[:])
	}
}

func TestIterator(t *testing.T) {
	testIterator(t, newRbtDBWithContext())
	testIterator(t, newArtDBWithContext())
}

func testIterator(t *testing.T, db MemBuffer) {
	assert := assert.New(t)
	const cnt = 10000
	fillDB(db, cnt)

	var buf [4]byte
	var i int

	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i++
	}
	assert.Equal(i, cnt)

	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i-1))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i--
	}
	assert.Equal(i, 0)

	var upperBoundBytes, lowerBoundBytes [4]byte
	bound := 400
	binary.BigEndian.PutUint32(upperBoundBytes[:], uint32(bound))
	for it, _ := db.Iter(nil, upperBoundBytes[:]); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i++
	}
	assert.Equal(i, bound)

	i = cnt
	binary.BigEndian.PutUint32(lowerBoundBytes[:], uint32(bound))
	for it, _ := db.IterReverse(nil, lowerBoundBytes[:]); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i-1))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i--
	}
	assert.Equal(i, bound)
}

func TestDiscard(t *testing.T) {
	testDiscard(t, newRbtDBWithContext())
	testDiscard(t, newArtDBWithContext())
}

func testDiscard(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const cnt = 10000
	base := deriveAndFill(db, 0, cnt, 0)
	sz := db.Size()

	db.Cleanup(deriveAndFill(db, 0, cnt, 1))
	assert.Equal(db.Len(), cnt)
	assert.Equal(db.Size(), sz)

	var buf [4]byte

	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		v, err := db.Get(context.Background(), buf[:])
		assert.Nil(err)
		assert.Equal(v, buf[:])
	}

	var i int
	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i++
	}
	assert.Equal(i, cnt)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i--
	}
	assert.Equal(i, -1)

	db.Cleanup(base)
	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		_, err := db.Get(context.Background(), buf[:])
		assert.NotNil(err)
	}
	it, _ := db.Iter(nil, nil)
	assert.False(it.Valid())
}

func TestFlushOverwrite(t *testing.T) {
	testFlushOverwrite(t, newRbtDBWithContext())
	testFlushOverwrite(t, newArtDBWithContext())
}

func testFlushOverwrite(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const cnt = 10000
	db.Release(deriveAndFill(db, 0, cnt, 0))
	sz := db.Size()

	db.Release(deriveAndFill(db, 0, cnt, 1))

	assert.Equal(db.Len(), cnt)
	assert.Equal(db.Size(), sz)

	var kbuf, vbuf [4]byte

	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		v, err := db.Get(context.Background(), kbuf[:])
		assert.Nil(err)
		assert.Equal(v, vbuf[:])
	}

	var i int
	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i++
	}
	assert.Equal(i, cnt)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i--
	}
	assert.Equal(i, -1)
}

func TestComplexUpdate(t *testing.T) {
	testComplexUpdate(t, newRbtDBWithContext())
	testComplexUpdate(t, newArtDBWithContext())
}

func testComplexUpdate(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const (
		keep      = 3000
		overwrite = 6000
		insert    = 9000
	)

	db.Release(deriveAndFill(db, 0, overwrite, 0))
	assert.Equal(db.Len(), overwrite)
	db.Release(deriveAndFill(db, keep, insert, 1))
	assert.Equal(db.Len(), insert)

	var kbuf, vbuf [4]byte

	for i := 0; i < insert; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i >= keep {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		v, err := db.Get(context.Background(), kbuf[:])
		assert.Nil(err)
		assert.Equal(v, vbuf[:])
	}
}

func TestNestedSandbox(t *testing.T) {
	testNestedSandbox(t, newRbtDBWithContext())
	testNestedSandbox(t, newArtDBWithContext())
}

func testNestedSandbox(t *testing.T, db MemBuffer) {
	assert := assert.New(t)
	h0 := deriveAndFill(db, 0, 200, 0)
	h1 := deriveAndFill(db, 0, 100, 1)
	h2 := deriveAndFill(db, 50, 150, 2)
	h3 := deriveAndFill(db, 100, 120, 3)
	h4 := deriveAndFill(db, 0, 150, 4)
	db.Cleanup(h4) // Discard (0..150 -> 4)
	db.Release(h3) // Flush (100..120 -> 3)
	db.Cleanup(h2) // Discard (100..120 -> 3) & (50..150 -> 2)
	db.Release(h1) // Flush (0..100 -> 1)
	db.Release(h0) // Flush (0..100 -> 1) & (0..200 -> 0)
	// The final result should be (0..100 -> 1) & (101..200 -> 0)

	var kbuf, vbuf [4]byte

	for i := 0; i < 200; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i < 100 {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		v, err := db.Get(context.Background(), kbuf[:])
		assert.Nil(err)
		assert.Equal(v, vbuf[:])
	}

	var i int

	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i < 100 {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i++
	}
	assert.Equal(i, 200)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i < 100 {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i--
	}
	assert.Equal(i, -1)
}

func TestOverwrite(t *testing.T) {
	testOverwrite(t, newRbtDBWithContext())
	testOverwrite(t, newArtDBWithContext())
}

func testOverwrite(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const cnt = 10000
	fillDB(db, cnt)
	var buf [4]byte

	sz := db.Size()
	for i := 0; i < cnt; i += 3 {
		var newBuf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		binary.BigEndian.PutUint32(newBuf[:], uint32(i*10))
		db.Set(buf[:], newBuf[:])
	}
	assert.Equal(db.Len(), cnt)
	assert.Equal(db.Size(), sz)

	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		val, _ := db.Get(context.Background(), buf[:])
		v := binary.BigEndian.Uint32(val)
		if i%3 == 0 {
			assert.Equal(v, uint32(i*10))
		} else {
			assert.Equal(v, uint32(i))
		}
	}

	var i int

	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		v := binary.BigEndian.Uint32(it.Value())
		if i%3 == 0 {
			assert.Equal(v, uint32(i*10))
		} else {
			assert.Equal(v, uint32(i))
		}
		i++
	}
	assert.Equal(i, cnt)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		v := binary.BigEndian.Uint32(it.Value())
		if i%3 == 0 {
			assert.Equal(v, uint32(i*10))
		} else {
			assert.Equal(v, uint32(i))
		}
		i--
	}
	assert.Equal(i, -1)
}

func TestReset(t *testing.T) {
	testReset(t, newRbtDBWithContext())
	testReset(t, newArtDBWithContext())
}

func testReset(t *testing.T, db interface {
	MemBuffer
	Reset()
}) {
	assert := assert.New(t)
	fillDB(db, 1000)
	db.Reset()
	_, err := db.Get(context.Background(), []byte{0, 0, 0, 0})
	assert.NotNil(err)
	_, err = db.GetFlags([]byte{0, 0, 0, 0})
	assert.NotNil(err)
	it, _ := db.Iter(nil, nil)
	assert.False(it.Valid())
}

func TestInspectStage(t *testing.T) {
	testInspectStage(t, newRbtDBWithContext())
	testInspectStage(t, newArtDBWithContext())
}

func testInspectStage(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	h1 := deriveAndFill(db, 0, 1000, 0)
	h2 := deriveAndFill(db, 500, 1000, 1)
	for i := 500; i < 1500; i++ {
		var kbuf [4]byte
		// don't update in place
		var vbuf [5]byte
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+2))
		db.Set(kbuf[:], vbuf[:])
	}
	h3 := deriveAndFill(db, 1000, 2000, 3)

	db.InspectStage(h3, func(key []byte, _ KeyFlags, val []byte) {
		k := int(binary.BigEndian.Uint32(key))
		v := int(binary.BigEndian.Uint32(val))

		assert.True(k >= 1000 && k < 2000)
		assert.Equal(v-k, 3)
	})

	db.InspectStage(h2, func(key []byte, _ KeyFlags, val []byte) {
		k := int(binary.BigEndian.Uint32(key))
		v := int(binary.BigEndian.Uint32(val))

		assert.True(k >= 500 && k < 2000)
		if k < 1000 {
			assert.Equal(v-k, 2)
		} else {
			assert.Equal(v-k, 3)
		}
	})

	db.Cleanup(h3)
	db.Release(h2)

	db.InspectStage(h1, func(key []byte, _ KeyFlags, val []byte) {
		k := int(binary.BigEndian.Uint32(key))
		v := int(binary.BigEndian.Uint32(val))

		assert.True(k >= 0 && k < 1500)
		if k < 500 {
			assert.Equal(v-k, 0)
		} else {
			assert.Equal(v-k, 2)
		}
	})

	db.Release(h1)
}

func TestDirty(t *testing.T) {
	testDirty(t, func() MemBuffer { return newRbtDBWithContext() })
	testDirty(t, func() MemBuffer { return newArtDBWithContext() })
}

func testDirty(t *testing.T, createDb func() MemBuffer) {
	assert := assert.New(t)

	db := createDb()
	db.Set([]byte{1}, []byte{1})
	assert.True(db.Dirty())

	db = createDb()
	h := db.Staging()
	db.Set([]byte{1}, []byte{1})
	db.Cleanup(h)
	assert.False(db.Dirty())

	h = db.Staging()
	db.Set([]byte{1}, []byte{1})
	db.Release(h)
	assert.True(db.Dirty())

	// persistent flags will make memdb dirty.
	db = createDb()
	h = db.Staging()
	db.SetWithFlags([]byte{1}, []byte{1}, kv.SetKeyLocked)
	db.Cleanup(h)
	assert.True(db.Dirty())

	// non-persistent flags will not make memdb dirty.
	db = createDb()
	h = db.Staging()
	db.SetWithFlags([]byte{1}, []byte{1}, kv.SetPresumeKeyNotExists)
	db.Cleanup(h)
	assert.False(db.Dirty())
}

func TestFlags(t *testing.T) {
	testFlags(t, newRbtDBWithContext(), func(db MemBuffer) Iterator { return db.(*rbtDBWithContext).IterWithFlags(nil, nil) })
	testFlags(t, newArtDBWithContext(), func(db MemBuffer) Iterator { return db.(*artDBWithContext).IterWithFlags(nil, nil) })

	testFlags(t, newRbtDBWithContext(), func(db MemBuffer) Iterator { return db.(*rbtDBWithContext).IterReverseWithFlags(nil) })
	testFlags(t, newArtDBWithContext(), func(db MemBuffer) Iterator { return db.(*artDBWithContext).IterReverseWithFlags(nil) })
}

func testFlags(t *testing.T, db MemBuffer, iterWithFlags func(db MemBuffer) Iterator) {
	assert := assert.New(t)

	const cnt = 10000
	h := db.Staging()
	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		if i%2 == 0 {
			db.SetWithFlags(buf[:], buf[:], kv.SetPresumeKeyNotExists, kv.SetKeyLocked)
		} else {
			db.SetWithFlags(buf[:], buf[:], kv.SetPresumeKeyNotExists)
		}
	}
	db.Cleanup(h)

	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		_, err := db.Get(context.Background(), buf[:])
		assert.NotNil(err)
		flags, err := db.GetFlags(buf[:])
		if i%2 == 0 {
			assert.Nil(err)
			assert.True(flags.HasLocked())
			assert.False(flags.HasPresumeKeyNotExists())
		} else {
			assert.NotNil(err)
		}
	}

	assert.Equal(db.Len(), 5000)
	assert.Equal(db.Size(), 20000)

	it, _ := db.Iter(nil, nil)
	assert.False(it.Valid())

	it = iterWithFlags(db)
	for ; it.Valid(); it.Next() {
		k := binary.BigEndian.Uint32(it.Key())
		assert.True(k%2 == 0)
		hasValue := it.(interface {
			HasValue() bool
		}).HasValue()
		assert.False(hasValue)
	}

	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		db.UpdateFlags(buf[:], kv.DelKeyLocked)
	}
	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		_, err := db.Get(context.Background(), buf[:])
		assert.NotNil(err)

		// UpdateFlags will create missing node.
		flags, err := db.GetFlags(buf[:])
		assert.Nil(err)
		assert.False(flags.HasLocked())
	}
}

func checkConsist(t *testing.T, p1 MemBuffer, p2 *leveldb.DB) {
	assert := assert.New(t)

	assert.Equal(p1.Len(), p2.Len())
	assert.Equal(p1.Size(), p2.Size())

	it1, _ := p1.Iter(nil, nil)
	it2 := p2.NewIterator(nil)

	var prevKey, prevVal []byte
	for it2.First(); it2.Valid(); it2.Next() {
		v, err := p1.Get(context.Background(), it2.Key())
		assert.Nil(err)
		assert.Equal(v, it2.Value())

		assert.Equal(it1.Key(), it2.Key())
		assert.Equal(it1.Value(), it2.Value())

		it, _ := p1.Iter(it2.Key(), nil)
		assert.Equal(it.Key(), it2.Key())
		assert.Equal(it.Value(), it2.Value())

		if prevKey != nil {
			it, _ = p1.IterReverse(it2.Key(), nil)
			assert.Equal(it.Key(), prevKey)
			assert.Equal(it.Value(), prevVal)
		}

		it1.Next()
		prevKey = it2.Key()
		prevVal = it2.Value()
	}

	it1, _ = p1.IterReverse(nil, nil)
	for it2.Last(); it2.Valid(); it2.Prev() {
		assert.Equal(it1.Key(), it2.Key())
		assert.Equal(it1.Value(), it2.Value())
		it1.Next()
	}
}

func fillDB(db MemBuffer, cnt int) {
	h := deriveAndFill(db, 0, cnt, 0)
	db.Release(h)
}

func deriveAndFill(db MemBuffer, start, end, valueBase int) int {
	h := db.Staging()
	var kbuf, vbuf [4]byte
	for i := start; i < end; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+valueBase))
		db.Set(kbuf[:], vbuf[:])
	}
	return h
}

const (
	startIndex = 0
	testCount  = 2
	indexStep  = 2
)

func insertData(t *testing.T, buffer MemBuffer) {
	for i := startIndex; i < testCount; i++ {
		val := encodeInt(i * indexStep)
		err := buffer.Set(val, val)
		assert.Nil(t, err)
	}
}

func encodeInt(n int) []byte {
	return []byte(fmt.Sprintf("%010d", n))
}

func decodeInt(s []byte) int {
	var n int
	fmt.Sscanf(string(s), "%010d", &n)
	return n
}

func valToStr(iter Iterator) string {
	val := iter.Value()
	return string(val)
}

func checkNewIterator(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	for i := startIndex; i < testCount; i++ {
		val := encodeInt(i * indexStep)
		iter, err := buffer.Iter(val, nil)
		assert.Nil(err)
		assert.Equal(iter.Key(), val)
		assert.Equal(decodeInt([]byte(valToStr(iter))), i*indexStep)
		iter.Close()
	}

	// Test iterator Next()
	for i := startIndex; i < testCount-1; i++ {
		val := encodeInt(i * indexStep)
		iter, err := buffer.Iter(val, nil)
		assert.Nil(err)
		assert.Equal(iter.Key(), val)
		assert.Equal(valToStr(iter), string(val))

		err = iter.Next()
		assert.Nil(err)
		assert.True(iter.Valid())

		val = encodeInt((i + 1) * indexStep)
		assert.Equal(iter.Key(), val)
		assert.Equal(valToStr(iter), string(val))
		iter.Close()
	}

	// Non exist and beyond maximum seek test
	iter, err := buffer.Iter(encodeInt(testCount*indexStep), nil)
	assert.Nil(err)
	assert.False(iter.Valid())

	// Non exist but between existing keys seek test,
	// it returns the smallest key that larger than the one we are seeking
	inBetween := encodeInt((testCount-1)*indexStep - 1)
	last := encodeInt((testCount - 1) * indexStep)
	iter, err = buffer.Iter(inBetween, nil)
	assert.Nil(err)
	assert.True(iter.Valid())
	assert.NotEqual(iter.Key(), inBetween)
	assert.Equal(iter.Key(), last)
	iter.Close()
}

func mustGet(t *testing.T, buffer MemBuffer) {
	for i := startIndex; i < testCount; i++ {
		s := encodeInt(i * indexStep)
		val, err := buffer.Get(context.Background(), s)
		assert.Nil(t, err)
		assert.Equal(t, string(val), string(s))
	}
}

func TestKVGetSet(t *testing.T) {
	testKVGetSet(t, newRbtDBWithContext())
	testKVGetSet(t, newArtDBWithContext())
}

func testKVGetSet(t *testing.T, buffer MemBuffer) {
	insertData(t, buffer)
	mustGet(t, buffer)
}

func TestNewIterator(t *testing.T) {
	testNewIterator(t, newRbtDBWithContext())
	testNewIterator(t, newArtDBWithContext())
}

func testNewIterator(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	// should be invalid
	iter, err := buffer.Iter(nil, nil)
	assert.Nil(err)
	assert.False(iter.Valid())

	insertData(t, buffer)
	checkNewIterator(t, buffer)
}

// FnKeyCmp is the function for iterator the keys
type FnKeyCmp func(key []byte) bool

// NextUntil applies FnKeyCmp to each entry of the iterator until meets some condition.
// It will stop when fn returns true, or iterator is invalid or an error occurs.
func NextUntil(it Iterator, fn FnKeyCmp) error {
	var err error
	for it.Valid() && !fn(it.Key()) {
		err = it.Next()
		if err != nil {
			return err
		}
	}
	return nil
}

func TestIterNextUntil(t *testing.T) {
	testIterNextUntil(t, newRbtDBWithContext())
	testIterNextUntil(t, newArtDBWithContext())
}

func testIterNextUntil(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	insertData(t, buffer)

	iter, err := buffer.Iter(nil, nil)
	assert.Nil(err)

	err = NextUntil(iter, func(k []byte) bool {
		return false
	})
	assert.Nil(err)
	assert.False(iter.Valid())
}

func TestBasicNewIterator(t *testing.T) {
	testBasicNewIterator(t, newRbtDBWithContext())
	testBasicNewIterator(t, newArtDBWithContext())
}

func testBasicNewIterator(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	it, err := buffer.Iter([]byte("2"), nil)
	assert.Nil(err)
	assert.False(it.Valid())
}

func TestNewIteratorMin(t *testing.T) {
	testNewIteratorMin(t, newRbtDBWithContext())
	testNewIteratorMin(t, newArtDBWithContext())
}

func testNewIteratorMin(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	kvs := []struct {
		key   string
		value string
	}{
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000001", "lock-version"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000001_0002", "1"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000001_0003", "hello"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000002", "lock-version"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000002_0002", "2"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000002_0003", "hello"},
	}
	for _, kv := range kvs {
		err := buffer.Set([]byte(kv.key), []byte(kv.value))
		assert.Nil(err)
	}

	cnt := 0
	it, err := buffer.Iter(nil, nil)
	assert.Nil(err)
	for it.Valid() {
		cnt++
		err := it.Next()
		assert.Nil(err)
	}
	assert.Equal(cnt, 6)

	it, err = buffer.Iter([]byte("DATA_test_main_db_tbl_tbl_test_record__00000000000000000000"), nil)
	assert.Nil(err)
	assert.Equal(string(it.Key()), "DATA_test_main_db_tbl_tbl_test_record__00000000000000000001")
}

func TestMemDBStaging(t *testing.T) {
	testMemDBStaging(t, newRbtDBWithContext())
	testMemDBStaging(t, newArtDBWithContext())
}

func testMemDBStaging(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	err := buffer.Set([]byte("x"), make([]byte, 2))
	assert.Nil(err)

	h1 := buffer.Staging()
	err = buffer.Set([]byte("x"), make([]byte, 3))
	assert.Nil(err)

	h2 := buffer.Staging()
	err = buffer.Set([]byte("yz"), make([]byte, 1))
	assert.Nil(err)

	v, _ := buffer.Get(context.Background(), []byte("x"))
	assert.Equal(len(v), 3)

	buffer.Release(h2)

	v, _ = buffer.Get(context.Background(), []byte("yz"))
	assert.Equal(len(v), 1)

	buffer.Cleanup(h1)

	v, _ = buffer.Get(context.Background(), []byte("x"))
	assert.Equal(len(v), 2)
}

func TestMemDBMultiLevelStaging(t *testing.T) {
	testMemDBMultiLevelStaging(t, newRbtDBWithContext())
	testMemDBMultiLevelStaging(t, newArtDBWithContext())
}

func testMemDBMultiLevelStaging(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)

	key := []byte{0}
	for i := 0; i < 100; i++ {
		assert.Equal(i+1, buffer.Staging())
		buffer.Set(key, []byte{byte(i)})
		v, err := buffer.Get(context.Background(), key)
		assert.Nil(err)
		assert.Equal(v, []byte{byte(i)})
	}

	for i := 99; i >= 0; i-- {
		expect := i
		if i%2 == 1 {
			expect = i - 1
			buffer.Cleanup(i + 1)
		} else {
			buffer.Release(i + 1)
		}
		v, err := buffer.Get(context.Background(), key)
		assert.Nil(err)
		assert.Equal(v, []byte{byte(expect)})
	}
}

func TestInvalidStagingHandle(t *testing.T) {
	testInvalidStagingHandle(t, newRbtDBWithContext())
	testInvalidStagingHandle(t, newArtDBWithContext())
}

func testInvalidStagingHandle(t *testing.T, buffer MemBuffer) {
	// handle == 0 takes no effect
	// MemBuffer.Release only accept the latest handle
	// MemBuffer.Cleanup accept handle large or equal than the latest handle, but only takes effect when handle is the latest handle.
	assert := assert.New(t)

	// test MemBuffer.Release
	h1 := buffer.Staging()
	assert.Positive(h1)
	h2 := buffer.Staging()
	assert.Positive(h2)
	assert.Panics(func() {
		buffer.Release(h2 + 1)
	})
	assert.Panics(func() {
		buffer.Release(h2 - 1)
	})
	buffer.Release(0)
	buffer.Release(h2)
	buffer.Release(0)
	buffer.Release(h1)
	buffer.Release(0)

	// test MemBuffer.Cleanup
	h1 = buffer.Staging()
	assert.Positive(h1)
	h2 = buffer.Staging()
	assert.Positive(h2)
	buffer.Cleanup(h2 + 1) // Cleanup is ok even if the handle is greater than the existing handles.
	assert.Panics(func() {
		buffer.Cleanup(h2 - 1)
	})
	buffer.Cleanup(0)
	buffer.Cleanup(h2)
	buffer.Cleanup(0)
	buffer.Cleanup(h1)
	buffer.Cleanup(0)
}

func TestMemDBCheckpoint(t *testing.T) {
	testMemDBCheckpoint(t, newRbtDBWithContext())
	testMemDBCheckpoint(t, newArtDBWithContext())
}

func testMemDBCheckpoint(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	cp1 := buffer.Checkpoint()

	buffer.Set([]byte("x"), []byte("x"))

	cp2 := buffer.Checkpoint()
	buffer.Set([]byte("y"), []byte("y"))

	h := buffer.Staging()
	buffer.Set([]byte("z"), []byte("z"))
	buffer.Release(h)

	for _, k := range []string{"x", "y", "z"} {
		v, _ := buffer.Get(context.Background(), []byte(k))
		assert.Equal(v, []byte(k))
	}

	buffer.RevertToCheckpoint(cp2)
	v, _ := buffer.Get(context.Background(), []byte("x"))
	assert.Equal(v, []byte("x"))
	for _, k := range []string{"y", "z"} {
		_, err := buffer.Get(context.Background(), []byte(k))
		assert.NotNil(err)
	}

	buffer.RevertToCheckpoint(cp1)
	_, err := buffer.Get(context.Background(), []byte("x"))
	assert.NotNil(err)
}

func TestBufferLimit(t *testing.T) {
	testBufferLimit(t, newRbtDBWithContext())
	testBufferLimit(t, newArtDBWithContext())
}

func testBufferLimit(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	buffer.SetEntrySizeLimit(500, 1000)

	err := buffer.Set([]byte("x"), make([]byte, 500))
	assert.NotNil(err) // entry size limit

	err = buffer.Set([]byte("x"), make([]byte, 499))
	assert.Nil(err)
	err = buffer.Set([]byte("yz"), make([]byte, 499))
	assert.NotNil(err) // buffer size limit

	err = buffer.Delete(make([]byte, 499))
	assert.Nil(err)

	err = buffer.Delete(make([]byte, 500))
	assert.NotNil(err)
}

func TestUnsetTemporaryFlag(t *testing.T) {
	testUnsetTemporaryFlag(t, newRbtDBWithContext())
	testUnsetTemporaryFlag(t, newArtDBWithContext())
}

func testUnsetTemporaryFlag(t *testing.T, buffer MemBuffer) {
	require := require.New(t)
	key := []byte{1}
	value := []byte{2}
	buffer.SetWithFlags(key, value, kv.SetNeedConstraintCheckInPrewrite)
	buffer.Set(key, value)
	flags, err := buffer.GetFlags(key)
	require.Nil(err)
	require.False(flags.HasNeedConstraintCheckInPrewrite())
}

func TestSnapshotGetIter(t *testing.T) {
	testSnapshotGetIter(t, newRbtDBWithContext())
	testSnapshotGetIter(t, newArtDBWithContext())
}

func testSnapshotGetIter(t *testing.T, db MemBuffer) {
	assert := assert.New(t)
	var getters []Getter
	var iters []Iterator
	var reverseIters []Iterator
	for i := 0; i < 100; i++ {
		assert.Nil(db.Set([]byte{byte(0)}, []byte{byte(i)}))
		assert.Nil(db.Set([]byte{byte(1)}, []byte{byte(i)}))

		// getter
		getter := db.SnapshotGetter()
		val, err := getter.Get(context.Background(), []byte{byte(0)})
		assert.Nil(err)
		assert.Equal(val, []byte{byte(min(i, 50))})
		getters = append(getters, getter)

		// iter
		iter := db.SnapshotIter(nil, nil)
		assert.Equal(iter.Key(), []byte{byte(0)})
		assert.Equal(iter.Value(), []byte{byte(min(i, 50))})
		iter.Close()
		iters = append(iters, db.SnapshotIter(nil, nil))

		// reverse iter
		reverseIter := db.SnapshotIterReverse(nil, nil)
		assert.Equal(reverseIter.Key(), []byte{byte(1)})
		assert.Equal(reverseIter.Value(), []byte{byte(min(i, 50))})
		reverseIter.Close()
		reverseIters = append(reverseIters, db.SnapshotIterReverse(nil, nil))

		// writes after staging should be bypassed in snapshot read.
		if i == 50 {
			_ = db.Staging()
		}
	}
	for _, getter := range getters {
		val, err := getter.Get(context.Background(), []byte{byte(0)})
		assert.Nil(err)
		assert.Equal(val, []byte{byte(50)})
	}
	for _, iter := range iters {
		assert.Equal(iter.Key(), []byte{byte(0)})
		assert.Equal(iter.Value(), []byte{byte(50)})
	}
	for _, reverseIter := range reverseIters {
		assert.Equal(reverseIter.Key(), []byte{byte(1)})
		assert.Equal(reverseIter.Value(), []byte{byte(50)})
	}

	db.(interface {
		Reset()
	}).Reset()
	db.UpdateFlags([]byte{255}, kv.SetPresumeKeyNotExists)
	// set (2, 2) ... (100, 100) in snapshot
	for i := 1; i < 50; i++ {
		db.Set([]byte{byte(2 * i)}, []byte{byte(2 * i)})
	}
	h := db.Staging()
	// set (0, 0) (1, 2) (2, 4) ... (100, 200) in staging
	for i := 0; i < 100; i++ {
		db.Set([]byte{byte(i)}, []byte{byte(2 * i)})
	}

	snapGetter := db.SnapshotGetter()
	v, err := snapGetter.Get(context.Background(), []byte{byte(2)})
	assert.Nil(err)
	assert.Equal(v, []byte{byte(2)})
	_, err = snapGetter.Get(context.Background(), []byte{byte(1)})
	assert.NotNil(err)
	_, err = snapGetter.Get(context.Background(), []byte{byte(254)})
	assert.NotNil(err)
	_, err = snapGetter.Get(context.Background(), []byte{byte(255)})
	assert.NotNil(err)

	it := db.SnapshotIter(nil, nil)
	// snapshot iter only see the snapshot data
	for i := 1; i < 50; i++ {
		assert.Equal(it.Key(), []byte{byte(2 * i)})
		assert.Equal(it.Value(), []byte{byte(2 * i)})
		assert.True(it.Valid())
		it.Next()
	}
	it = db.SnapshotIterReverse(nil, nil)
	for i := 49; i >= 1; i-- {
		assert.Equal(it.Key(), []byte{byte(2 * i)})
		assert.Equal(it.Value(), []byte{byte(2 * i)})
		assert.True(it.Valid())
		it.Next()
	}
	assert.False(it.Valid())
	db.Release(h)
}

func TestCleanupKeepPersistentFlag(t *testing.T) {
	testCleanupKeepPersistentFlag(t, newRbtDBWithContext())
	testCleanupKeepPersistentFlag(t, newArtDBWithContext())
}

func testCleanupKeepPersistentFlag(t *testing.T, db MemBuffer) {
	assert := assert.New(t)
	persistentFlag := kv.SetKeyLocked
	nonPersistentFlag := kv.SetPresumeKeyNotExists

	h := db.Staging()
	db.SetWithFlags([]byte{1}, []byte{1}, persistentFlag)
	db.SetWithFlags([]byte{2}, []byte{2}, nonPersistentFlag)
	db.SetWithFlags([]byte{3}, []byte{3}, persistentFlag, nonPersistentFlag)
	db.Cleanup(h)

	for _, key := range [][]byte{{1}, {2}, {3}} {
		// the values are reverted by MemBuffer.Cleanup
		_, err := db.Get(context.Background(), key)
		assert.NotNil(err)
	}

	flag, err := db.GetFlags([]byte{1})
	assert.Nil(err)
	assert.True(flag.HasLocked())
	_, err = db.GetFlags([]byte{2})
	assert.NotNil(err)
	flag, err = db.GetFlags([]byte{3})
	assert.Nil(err)
	assert.True(flag.HasLocked())
	assert.False(flag.HasPresumeKeyNotExists())
}

func TestIterNoResult(t *testing.T) {
	testIterNoResult(t, newRbtDBWithContext())
	testIterNoResult(t, newArtDBWithContext())
}

func testIterNoResult(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)

	assert.Nil(buffer.Set([]byte{1, 1}, []byte{1, 1}))

	checkFn := func(lowerBound, upperBound []byte) {
		iter, err := buffer.Iter(lowerBound, upperBound)
		assert.Nil(err)
		assert.False(iter.Valid())
		iter, err = buffer.IterReverse(upperBound, lowerBound)
		assert.Nil(err)
		assert.False(iter.Valid())
	}

	// Test lower bound and upper bound seek to the same position
	checkFn([]byte{1, 1}, []byte{1, 1})
	checkFn([]byte{1, 0, 0}, []byte{1, 0, 1})
	// Test lower bound > upper bound
	checkFn([]byte{1, 0, 1}, []byte{1, 0, 0})
}

func TestMemBufferCache(t *testing.T) {
	testMemBufferCache(t, newRbtDBWithContext())
	testMemBufferCache(t, newArtDBWithContext())
}

func testMemBufferCache(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)

	type CacheStats interface {
		GetCacheHitCount() uint64
		GetCacheMissCount() uint64
	}
	cacheCheck := func(hit bool, fn func()) {
		beforeHit, beforeMiss := buffer.(CacheStats).GetCacheHitCount(), buffer.(CacheStats).GetCacheMissCount()
		fn()
		afterHit, afterMiss := buffer.(CacheStats).GetCacheHitCount(), buffer.(CacheStats).GetCacheMissCount()
		hitCnt := afterHit - beforeHit
		missCnt := afterMiss - beforeMiss
		if hit {
			assert.Equal(hitCnt, uint64(1))
			assert.Equal(missCnt, uint64(0))
		} else {
			assert.Equal(hitCnt, uint64(0))
			assert.Equal(missCnt, uint64(1))
		}
	}

	cacheCheck(false, func() {
		assert.Nil(buffer.Set([]byte{1}, []byte{0}))
	})
	cacheCheck(true, func() {
		assert.Nil(buffer.Set([]byte{1}, []byte{1}))
	})
	cacheCheck(false, func() {
		assert.Nil(buffer.Set([]byte{2}, []byte{2}))
	})
	cacheCheck(true, func() {
		v, err := buffer.Get(context.Background(), []byte{2})
		assert.Nil(err)
		assert.Equal(v, []byte{2})
	})
	cacheCheck(false, func() {
		v, err := buffer.Get(context.Background(), []byte{1})
		assert.Nil(err)
		assert.Equal(v, []byte{1})
	})
	cacheCheck(true, func() {
		v, err := buffer.Get(context.Background(), []byte{1})
		assert.Nil(err)
		assert.Equal(v, []byte{1})
	})
	cacheCheck(false, func() {
		v, err := buffer.Get(context.Background(), []byte{2})
		assert.Nil(err)
		assert.Equal(v, []byte{2})
	})
	cacheCheck(true, func() {
		assert.Nil(buffer.Set([]byte{2}, []byte{2, 2}))
	})
	cacheCheck(true, func() {
		v, err := buffer.Get(context.Background(), []byte{2})
		assert.Nil(err)
		assert.Equal(v, []byte{2, 2})
	})
}

func TestMemDBLeafFragmentation(t *testing.T) {
	testMemDBLeafFragmentation(t, newRbtDBWithContext())
	testMemDBLeafFragmentation(t, newArtDBWithContext())
}

func testMemDBLeafFragmentation(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	h := buffer.Staging()
	mem := buffer.Mem()
	for i := 0; i < 10; i++ {
		for k := 0; k < 100; k++ {
			buffer.Set([]byte(strings.Repeat(strconv.Itoa(k), 256)), []byte("value"))
		}
		cur := buffer.Mem()
		if mem == 0 {
			mem = cur
		} else {
			assert.LessOrEqual(cur, mem)
		}
		buffer.Cleanup(h)
		h = buffer.Staging()
	}
}

func TestReadOnlyZeroMem(t *testing.T) {
	// read only MemBuffer should not allocate heap memory.
	assert.Zero(t, newRbtDBWithContext().Mem())
	assert.Zero(t, newArtDBWithContext().Mem())
}

func TestKeyValueOversize(t *testing.T) {
	check := func(t *testing.T, db MemBuffer) {
		key := make([]byte, math.MaxUint16)
		overSizeKey := make([]byte, math.MaxUint16+1)

		assert.Nil(t, db.Set(key, overSizeKey))
		err := db.Set(overSizeKey, key)
		assert.NotNil(t, err)
		assert.Equal(t, err.(*tikverr.ErrKeyTooLarge).KeySize, math.MaxUint16+1)
	}

	check(t, newRbtDBWithContext())
	check(t, newArtDBWithContext())
}

func TestSetMemoryFootprintChangeHook(t *testing.T) {
	check := func(t *testing.T, db MemBuffer) {
		memoryConsumed := uint64(0)
		assert.False(t, db.MemHookSet())
		db.SetMemoryFootprintChangeHook(func(mem uint64) {
			memoryConsumed = mem
		})
		assert.True(t, db.MemHookSet())

		assert.Zero(t, memoryConsumed)
		db.Set([]byte{1}, []byte{1})
		assert.NotZero(t, memoryConsumed)
	}

	check(t, newRbtDBWithContext())
	check(t, newArtDBWithContext())
}

func TestSelectValueHistory(t *testing.T) {
	check := func(t *testing.T, db interface {
		MemBuffer
		SelectValueHistory(key []byte, predicate func(value []byte) bool) ([]byte, error)
	}) {
		db.Set([]byte{1}, []byte{1})
		h := db.Staging()
		db.Set([]byte{1}, []byte{1, 1})

		val, err := db.SelectValueHistory([]byte{1}, func(value []byte) bool { return bytes.Equal(value, []byte{1}) })
		assert.Nil(t, err)
		assert.Equal(t, val, []byte{1})
		val, err = db.SelectValueHistory([]byte{1}, func(value []byte) bool { return bytes.Equal(value, []byte{1, 1}) })
		assert.Nil(t, err)
		assert.Equal(t, val, []byte{1, 1})
		val, err = db.SelectValueHistory([]byte{1}, func(value []byte) bool { return bytes.Equal(value, []byte{1, 1, 1}) })
		assert.Nil(t, err)
		assert.Nil(t, val)
		_, err = db.SelectValueHistory([]byte{2}, func([]byte) bool { return false })
		assert.NotNil(t, err)

		db.Cleanup(h)

		val, err = db.SelectValueHistory([]byte{1}, func(value []byte) bool { return bytes.Equal(value, []byte{1}) })
		assert.Nil(t, err)
		assert.Equal(t, val, []byte{1})
		val, err = db.SelectValueHistory([]byte{1}, func(value []byte) bool { return bytes.Equal(value, []byte{1, 1}) })
		assert.Nil(t, err)
		assert.Nil(t, val)
	}

	check(t, newRbtDBWithContext())
	check(t, newArtDBWithContext())
}

func TestSnapshotReaderWithWrite(t *testing.T) {
	check := func(db MemBuffer, num int) {
		for i := 0; i < num; i++ {
			db.Set([]byte{0, byte(i)}, []byte{0, byte(i)})
		}
		h := db.Staging()
		defer db.Release(h)

		iter := db.SnapshotIter([]byte{0, 0}, []byte{0, 255})
		assert.Equal(t, iter.Key(), []byte{0, 0})

		db.Set([]byte{0, byte(num)}, []byte{0, byte(num)}) // ART: node4/node16/node48 is freed and wait to be reused.

		// ART: reuse the node4/node16/node48
		for i := 0; i < num; i++ {
			db.Set([]byte{1, byte(i)}, []byte{1, byte(i)})
		}

		for i := 0; i < num; i++ {
			assert.True(t, iter.Valid())
			assert.Equal(t, iter.Key(), []byte{0, byte(i)})
			assert.Nil(t, iter.Next())
		}
		assert.False(t, iter.Valid())
		iter.Close()
	}

	check(newRbtDBWithContext(), 4)
	check(newArtDBWithContext(), 4)

	check(newRbtDBWithContext(), 16)
	check(newArtDBWithContext(), 16)

	check(newRbtDBWithContext(), 48)
	check(newArtDBWithContext(), 48)
}

func TestBatchedSnapshotIter(t *testing.T) {
	check := func(db *artDBWithContext, num int) {
		// Insert test data
		for i := 0; i < num; i++ {
			db.Set([]byte{0, byte(i)}, []byte{0, byte(i)})
		}
		h := db.Staging()
		defer db.Release(h)
		snapshot := db.GetSnapshot()
		defer snapshot.Close()

		// Create iterator - should be positioned at first key
		iter := snapshot.BatchedSnapshotIter([]byte{0, 0}, []byte{0, 255}, false)
		defer iter.Close()

		// Should be able to read first key immediately
		require.True(t, iter.Valid())
		require.Equal(t, []byte{0, 0}, iter.Key())

		// Write additional data
		db.Set([]byte{0, byte(num)}, []byte{0, byte(num)})
		for i := 0; i < num; i++ {
			db.Set([]byte{1, byte(i)}, []byte{1, byte(i)})
		}

		// Verify iteration
		i := 0
		for ; i < num; i++ {
			require.True(t, iter.Valid())
			require.Equal(t, []byte{0, byte(i)}, iter.Key())
			require.Equal(t, []byte{0, byte(i)}, iter.Value())
			require.NoError(t, iter.Next())
		}
		require.False(t, iter.Valid())
	}

	checkReverse := func(db *artDBWithContext, num int) {
		for i := 0; i < num; i++ {
			db.Set([]byte{0, byte(i)}, []byte{0, byte(i)})
		}
		h := db.Staging()
		defer db.Release(h)
		snapshot := db.GetSnapshot()
		defer snapshot.Close()

		iter := snapshot.BatchedSnapshotIter([]byte{0, 0}, []byte{0, 255}, true)
		defer iter.Close()

		// Should be positioned at last key
		require.True(t, iter.Valid())
		require.Equal(t, []byte{0, byte(num - 1)}, iter.Key())

		db.Set([]byte{0, byte(num)}, []byte{0, byte(num)})
		for i := 0; i < num; i++ {
			db.Set([]byte{1, byte(i)}, []byte{1, byte(i)})
		}

		i := num - 1
		for ; i >= 0; i-- {
			require.True(t, iter.Valid())
			require.Equal(t, []byte{0, byte(i)}, iter.Key())
			require.Equal(t, []byte{0, byte(i)}, iter.Value())
			require.NoError(t, iter.Next())
		}
		require.False(t, iter.Valid())
	}

	// Run size test cases
	check(newArtDBWithContext(), 3)
	check(newArtDBWithContext(), 17)
	check(newArtDBWithContext(), 64)

	checkReverse(newArtDBWithContext(), 3)
	checkReverse(newArtDBWithContext(), 17)
	checkReverse(newArtDBWithContext(), 64)
}

func TestBatchedSnapshotIterEdgeCase(t *testing.T) {
	t.Run("EdgeCases", func(t *testing.T) {
		db := newArtDBWithContext()
		h := db.Staging()
		snapshot := db.GetSnapshot()
		// invalid range - should be invalid immediately
		iter := snapshot.BatchedSnapshotIter([]byte{1}, []byte{1}, false)
		require.False(t, iter.Valid())
		iter.Close()

		// empty range - should be invalid immediately
		iter = snapshot.BatchedSnapshotIter([]byte{0}, []byte{1}, false)
		require.False(t, iter.Valid())
		iter.Close()
		snapshot.Close()

		// Single element range
		_ = db.Set([]byte{1}, []byte{1})
		db.Release(h)
		h = db.Staging()
		snapshot = db.GetSnapshot()
		iter = snapshot.BatchedSnapshotIter([]byte{1}, []byte{2}, false)
		require.True(t, iter.Valid())
		require.Equal(t, []byte{1}, iter.Key())
		require.NoError(t, iter.Next())
		require.False(t, iter.Valid())
		iter.Close()

		// Multiple elements
		_ = db.Set([]byte{2}, []byte{2})
		_ = db.Set([]byte{3}, []byte{3})
		_ = db.Set([]byte{4}, []byte{4})
		snapshot.Close()
		db.Release(h)
		_ = db.Staging()

		// Forward iteration [2,4)
		snapshot = db.GetSnapshot()
		iter = snapshot.BatchedSnapshotIter([]byte{2}, []byte{4}, false)
		vals := []byte{}
		for iter.Valid() {
			vals = append(vals, iter.Key()[0])
			require.NoError(t, iter.Next())
		}
		require.Equal(t, []byte{2, 3}, vals)
		iter.Close()

		// Reverse iteration [2,4)
		iter = snapshot.BatchedSnapshotIter([]byte{2}, []byte{4}, true)
		vals = []byte{}
		for iter.Valid() {
			vals = append(vals, iter.Key()[0])
			require.NoError(t, iter.Next())
		}
		require.Equal(t, []byte{3, 2}, vals)
		iter.Close()
	})

	t.Run("BoundaryTests", func(t *testing.T) {
		db := newArtDBWithContext()
		keys := [][]byte{
			{1, 0}, {1, 2}, {1, 4}, {1, 6}, {1, 8},
		}
		for _, k := range keys {
			_ = db.Set(k, k)
		}

		// lower bound included
		h := db.Staging()
		defer db.Release(h)
		snapshot := db.GetSnapshot()
		defer snapshot.Close()
		iter := snapshot.BatchedSnapshotIter([]byte{1, 2}, []byte{1, 9}, false)
		vals := []byte{}
		for iter.Valid() {
			vals = append(vals, iter.Key()[1])
			require.NoError(t, iter.Next())
		}
		require.Equal(t, []byte{2, 4, 6, 8}, vals)
		iter.Close()

		// upper bound excluded
		iter = snapshot.BatchedSnapshotIter([]byte{1, 0}, []byte{1, 6}, false)
		vals = []byte{}
		for iter.Valid() {
			vals = append(vals, iter.Key()[1])
			require.NoError(t, iter.Next())
		}
		require.Equal(t, []byte{0, 2, 4}, vals)
		iter.Close()

		// reverse
		iter = snapshot.BatchedSnapshotIter([]byte{1, 0}, []byte{1, 6}, true)
		vals = []byte{}
		for iter.Valid() {
			vals = append(vals, iter.Key()[1])
			require.NoError(t, iter.Next())
		}
		require.Equal(t, []byte{4, 2, 0}, vals)
		iter.Close()
	})

	t.Run("AlphabeticalOrder", func(t *testing.T) {
		db := newArtDBWithContext()
		keys := [][]byte{
			{2},
			{2, 1},
			{2, 1, 1},
			{2, 1, 1, 1},
		}
		for _, k := range keys {
			_ = db.Set(k, k)
		}
		h := db.Staging()
		defer db.Release(h)
		snapshot := db.GetSnapshot()
		defer snapshot.Close()
		// forward
		iter := snapshot.BatchedSnapshotIter([]byte{2}, []byte{3}, false)
		count := 0
		for iter.Valid() {
			require.Equal(t, keys[count], iter.Key())
			require.NoError(t, iter.Next())
			count++
		}
		require.Equal(t, len(keys), count)
		iter.Close()

		// reverse
		iter = snapshot.BatchedSnapshotIter([]byte{2}, []byte{3}, true)
		count = len(keys) - 1
		for iter.Valid() {
			require.Equal(t, keys[count], iter.Key())
			require.NoError(t, iter.Next())
			count--
		}
		require.Equal(t, -1, count)
		iter.Close()
	})

	t.Run("BatchSizeGrowth", func(t *testing.T) {
		db := newArtDBWithContext()

		for i := 0; i < 100; i++ {
			_ = db.Set([]byte{3, byte(i)}, []byte{3, byte(i)})
		}
		h := db.Staging()
		defer db.Release(h)
		snapshot := db.GetSnapshot()
		defer snapshot.Close()
		// forward
		iter := snapshot.BatchedSnapshotIter([]byte{3, 0}, []byte{3, 255}, false)
		count := 0
		for iter.Valid() {
			require.Equal(t, []byte{3, byte(count)}, iter.Key())
			require.NoError(t, iter.Next())
			count++
		}
		require.Equal(t, 100, count)
		iter.Close()

		// reverse
		iter = snapshot.BatchedSnapshotIter([]byte{3, 0}, []byte{3, 255}, true)
		count = 99
		for iter.Valid() {
			require.Equal(t, []byte{3, byte(count)}, iter.Key())
			require.NoError(t, iter.Next())
			count--
		}
		require.Equal(t, -1, count)
		iter.Close()
	})

	t.Run("SnapshotChange", func(t *testing.T) {
		db := newArtDBWithContext()
		_ = db.Set([]byte{0}, []byte{0})
		h := db.Staging()
		snapshot := db.GetSnapshot()
		defer snapshot.Close()
		_ = db.Set([]byte{byte(1)}, []byte{byte(1)})
		iter := snapshot.BatchedSnapshotIter([]byte{0}, []byte{255}, false)
		require.True(t, iter.Valid())
		require.NoError(t, iter.Next())
		db.Release(h)
		db.Staging()
		require.False(t, iter.Valid())
		require.Error(t, iter.Next())
	})
}
