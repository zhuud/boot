package util

// slices / maps（Go 1.21+；Keys/Values 1.23+ 为 iterator）
//
//	方法                                作用                                  用法
//	Contains / ContainsFunc             是否包含该元素                        slices.Contains(s, v)
//	Index / IndexFunc                   第一次出现的下标，没有则 -1           slices.Index(s, v)
//	Equal / EqualFunc                   两个切片是否逐元素相等                slices.Equal(a, b)
//	Compare / CompareFunc               字典序：<0 小于、0 相等、>0 大于      slices.Compare(a, b)
//	Max / Min                           最大 / 最小元素；空切片会 panic       slices.Max(s)
//	Sort / Reverse                      原地排序 / 原地反转                   slices.Sort(s)
//	IsSorted                            是否已经有序                          slices.IsSorted(s)
//	BinarySearch                        在有序切片里二分查找                  i, ok := slices.BinarySearch(s, v)
//	Clone                               浅拷贝；nil 切片仍是 nil              s2 := slices.Clone(s)
//	Concat                              按顺序拼接                            s = slices.Concat(a, b)
//	Insert / Delete / Replace           插入、删除、替换；必须 s = 接返回值   s = slices.Insert(s, i, x)
//	Compact                             只去掉相邻重复；必须 s = 接返回值     s = slices.Compact(s)
//	Clip / Grow                         把 cap 收到 len / 预留多余 cap        s = slices.Clip(s)
//	Collect / Chunk                     把 iterator 收成切片 / 按长度分块     slices.Collect(seq)
//	maps.Equal / Clone                  键值是否全等 / 浅拷贝                 m2 := maps.Clone(m)
//	maps.Copy                           把 src 写入 dst，相同 key 覆盖        maps.Copy(dst, src)
//	maps.DeleteFunc                     按条件删除条目                        maps.DeleteFunc(m, fn)
//	maps.Keys / Values                  无序遍历 key / value（Go 1.23+）      slices.Collect(maps.Keys(m))
//
// 本文件：SliceDiff 差集，SliceIntersect 交集，SliceUnique 保序去重。

// SliceDiff 返回 first 中未出现在 others 里的元素。结果去重且不保证顺序。
func SliceDiff[V comparable](first []V, others ...[]V) []V {
	if len(first) == 0 {
		return nil
	}
	remaining := make(map[V]struct{}, len(first))
	for _, value := range first {
		remaining[value] = struct{}{}
	}
	for _, other := range others {
		for _, value := range other {
			delete(remaining, value)
		}
		if len(remaining) == 0 {
			return nil
		}
	}
	result := make([]V, 0, len(remaining))
	for value := range remaining {
		result = append(result, value)
	}
	return result
}

// SliceIntersect 返回所有输入切片共有的元素。无 others 时返回 first 的去重结果。
// 结果不保证顺序。
func SliceIntersect[V comparable](first []V, others ...[]V) []V {
	if len(first) == 0 {
		return nil
	}
	remaining := make(map[V]struct{}, len(first))
	for _, value := range first {
		remaining[value] = struct{}{}
	}
	for _, other := range others {
		next := make(map[V]struct{})
		for _, value := range other {
			if _, ok := remaining[value]; ok {
				next[value] = struct{}{}
			}
		}
		remaining = next
		if len(remaining) == 0 {
			return nil
		}
	}
	result := make([]V, 0, len(remaining))
	for value := range remaining {
		result = append(result, value)
	}
	return result
}

// SliceUnique 按首次出现顺序返回去重后的切片。默认保留零值；传入 skipZero=true 时丢弃零值。
func SliceUnique[V comparable](values []V, skipZero ...bool) []V {
	if len(values) == 0 {
		return nil
	}
	result := make([]V, 0, len(values))
	seen := make(map[V]struct{}, len(values))
	shouldSkipZero := len(skipZero) > 0 && skipZero[0]
	var zero V
	for _, value := range values {
		if shouldSkipZero && value == zero {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
