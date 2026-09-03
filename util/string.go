package util


// strings / strconv
//
//	方法                                作用                                  用法
//	Contains                            是否包含子串                          strings.Contains(s, sub)
//	Index                               子串第一次出现的下标，没有则 -1       strings.Index(s, sub)
//	HasPrefix / HasSuffix               是否具有指定前缀 / 后缀               strings.HasPrefix(s, p)
//	EqualFold                           忽略大小写后是否相等                  strings.EqualFold(a, b)
//	Compare                             字典序：<0 小于、0 相等、>0 大于      strings.Compare(a, b)
//	Cut                                 按第一次出现的 sep 切成两段           a, b, ok := strings.Cut(s, sep)
//	Split / Fields                      按 sep 切开 / 按空白切开              strings.Split(s, ",")
//	Lines / SplitSeq                    按行或 sep 迭代，比 Split 少分配      for line := range strings.Lines(s)
//	ReplaceAll                          把 old 全部换成 new                   strings.ReplaceAll(s, old, new)
//	TrimSpace                           去掉两端 Unicode 空白                 strings.TrimSpace(s)
//	ToLower / ToUpper                   转成小写 / 大写                       strings.ToLower(s)
//	Map                                 逐个字符变换；函数返回负值则删掉      strings.Map(fn, s)
//	Join                                用 sep 把字符串切片拼成一个           strings.Join(ss, ",")
//	strings.Builder                     高效拼接；必须用指针，值拷贝会丢数据  var b strings.Builder
//	Atoi / Itoa                         十进制字符串与 int 互转               n, err := strconv.Atoi(s)
//	ParseInt / FormatInt                按指定进制解析 / 格式化整数           strconv.ParseInt(s, 10, 64)
//	Quote                               加成 Go 双引号字面量                  strconv.Quote(s)
//	Unquote                             解析 Go 字面量，入参必须带引号        raw, err := strconv.Unquote(s)
//	s[i:j]                              按字节截取，越界会 panic              s[1:3]
//	utf8.RuneCountInString              Unicode 字符数，不是字节数 len(s)     utf8.RuneCountInString(s)
