package sandbox

import (
	"bytes"

	"github.com/xuperchain/xupercore/kernel/contract"
)

// multiIterator 按照归并排序合并两个XMIterator
// 如果两个XMIterator在某次迭代返回同样的Key，选取front的Value
type multiIterator struct {
	front contract.XMIterator
	back  contract.XMIterator

	first    bool
	frontEnd bool
	backEnd  bool

	key   []byte
	value *contract.VersionedData
}

func newMultiIterator(front, back contract.XMIterator) contract.XMIterator {
	m := &multiIterator{
		front: front,
		back:  back,
		first: true,
	}
	m.frontEnd = m.front.Next()
	m.backEnd = m.back.Next()
	k1, k2 := m.front.Key(), m.back.Key()
	ret := compareBytes(k1, k2)
	switch ret {
	case 0, -1:
		m.setKeyValue(m.front)
	case 1:
		m.setKeyValue(m.back)
	}
	return m
}

func (m *multiIterator) Key() []byte {
	if m.frontEnd && m.backEnd {
		return nil
	}
	return m.key
}

func (m *multiIterator) Value() *contract.VersionedData {
	if m.frontEnd && m.backEnd {
		return nil
	}
	return m.value
}

func (m *multiIterator) Next() bool {
	if m.frontEnd && m.backEnd {
		return false
	}
	if m.first {
		m.first = false
		return true
	}

	k1, k2 := m.front.Key(), m.back.Key()
	ret := compareBytes(k1, k2)
	switch ret {
	case 0:
		m.frontEnd = m.front.Next()
		m.backEnd = m.back.Next()
		m.setKeyValue(m.front)
	case -1:
		m.frontEnd = m.front.Next()
		m.setKeyValue(m.front)
	case 1:
		m.backEnd = m.back.Next()
		m.setKeyValue(m.back)
	default:
		panic("unexpected compareBytes return")
	}

	return !(m.frontEnd && m.backEnd)
}

func (m *multiIterator) setKeyValue(iter contract.XMIterator) {
	m.key = iter.Key()
	m.value = iter.Value()
}

func (m *multiIterator) Error() error {
	err := m.front.Error()
	if err != nil {
		return err
	}

	err = m.back.Error()
	if err != nil {
		return err
	}
	return nil
}

// Iterator 必须在使用完毕后关闭
func (m *multiIterator) Close() {
	m.front.Close()
	m.back.Close()
}

// rsetIterator 把迭代到的Key记录到读集里面
type rsetIterator struct {
	mc *XMCache
	contract.XMIterator
	err error
}

func newRsetIterator(iter contract.XMIterator, mc *XMCache) contract.XMIterator {
	return &rsetIterator{
		mc:         mc,
		XMIterator: iter,
	}
}

func (r *rsetIterator) Next() bool {
	if r.err != nil {
		return false
	}
	ok := r.XMIterator.Next()
	if !ok {
		return false
	}
	rawkey := r.Key()
	bucket, key, err := parseRawKey(rawkey)
	if err != nil {
		r.err = err
		return false
	}
	// fill read set
	r.mc.Get(bucket, key)
	return true
}

func (r *rsetIterator) Error() error {
	if r.err != nil {
		return r.err
	}
	return r.XMIterator.Error()
}

// // memIterator 把leveldb的Iterator转换成XMIterator
// type memIterator struct {
// 	mc *XMCache
// 	iterator.Iterator
// }

// func newMemIterator(iter iterator.Iterator, mc *XMCache) contract.XMIterator {
// 	return &memIterator{
// 		mc:       mc,
// 		Iterator: iter,
// 	}
// }

// func (m *memIterator) Value() *contract.VersionedData {
// 	return m.mc.getRawData(m.Iterator.Value())
// }

// // Iterator 必须在使用完毕后关闭
// func (m *memIterator) Close() {
// 	m.Release()
// }

// ContractIterator 把contract.XMIterator转换成contract.Iterator
type ContractIterator struct {
	contract.XMIterator
}

func newContractIterator(xmiter contract.XMIterator) contract.Iterator {
	return &ContractIterator{
		XMIterator: xmiter,
	}
}

func (c *ContractIterator) Value() []byte {
	v := c.XMIterator.Value()
	return v.PureData.Value
}

// stripDelIterator 从迭代器里剔除删除标注和空版本
type stripDelIterator struct {
	contract.XMIterator
}

func newStripDelIterator(xmiter contract.XMIterator) contract.XMIterator {
	return &stripDelIterator{
		XMIterator: xmiter,
	}
}

func (s *stripDelIterator) Next() bool {
	for s.XMIterator.Next() {
		v := s.Value()
		if IsEmptyVersionedData(v) {
			continue
		}
		if IsDelFlag(v.PureData.Value) {
			continue
		}
		return true
	}
	return false
}

// compareBytes like bytes.Compare but treats nil as max value
func compareBytes(k1, k2 []byte) int {
	if k1 == nil && k2 == nil {
		return 0
	}
	if k1 == nil {
		return 1
	}
	if k2 == nil {
		return -1
	}
	return bytes.Compare(k1, k2)
}
