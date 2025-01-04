package domain

import (
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

// PinyinDomainService 用于检测域名是否为拼音域名
type PinyinDomainService struct {
	// pinyinArgs 拼音转换的参数配置
	pinyinArgs pinyin.Args
	// 常用拼音词组的最小和最大长度
	minPinyinLen int
	maxPinyinLen int
}

// NewPinyinDomainService 创建一个新的拼音域名服务
func NewPinyinDomainService() *PinyinDomainService {
	// 初始化拼音转换参数
	args := pinyin.NewArgs()
	// 使用声母韵母风格
	args.Style = pinyin.Normal
	// 启用分词
	args.Separator = ""

	return &PinyinDomainService{
		pinyinArgs:   args,
		minPinyinLen: 2,    // 最短的拼音（如"ai"）
		maxPinyinLen: 6,    // 最长的拼音（如"zhuang"）
	}
}

// IsPinyinDomain 检查域名是否为拼音域名
func (s *PinyinDomainService) IsPinyinDomain(domain string) bool {
	// 检查是否全是字母
	if !isAllLetters(domain) {
		return false
	}

	// 排除单个字母的情况
	if len(domain) < 2 {
		return false
	}
	
	// 检查是否为拼音缩写
	if len(domain) <= 4 { // 通常拼音缩写不会太长
		if s.isValidPinyinAbbr(domain) {
			return true
		}
	}

	// 检查是否为拼音组合
	return s.isValidPinyinCombination(domain)
}

// isValidPinyinAbbr 检查是否为有效的拼音缩写
func (s *PinyinDomainService) isValidPinyinAbbr(word string) bool {
	// 必须至少两个字母
	if len(word) < 2 {
		return false
	}

	// 检查每个字母是否都是有效的拼音声母
	for _, c := range word {
		if !isPinyinInitial(string(c)) {
			return false
		}
	}
	return true
}

// isValidPinyinCombination 检查是否为有效的拼音组合
func (s *PinyinDomainService) isValidPinyinCombination(word string) bool {
	// 如果长度太短或太长，可能不是拼音
	if len(word) < 2 || len(word) > 30 {
		return false
	}

	// 尝试将词分割成可能的拼音片段
	return s.canBeSplitIntoPinyin(word)
}

// canBeSplitIntoPinyin 检查字符串是否可以被分割成有效的拼音组合
func (s *PinyinDomainService) canBeSplitIntoPinyin(word string) bool {
	if word == "" {
		return true
	}

	// 尝试不同长度的前缀
	for i := s.minPinyinLen; i <= s.maxPinyinLen && i <= len(word); i++ {
		prefix := word[:i]
		// 检查这个前缀是否是有效的拼音
		if s.isValidSinglePinyin(prefix) {
			// 递归检查剩余部分
			if s.canBeSplitIntoPinyin(word[i:]) {
				return true
			}
		}
	}
	return false
}

// isValidSinglePinyin 检查是否为有效的单个拼音
func (s *PinyinDomainService) isValidSinglePinyin(pinyin string) bool {
	// 常见的拼音音节
	validPinyins := map[string]bool{
		// 声母 b
		"ba": true, "bo": true, "bi": true, "bu": true, "bai": true, "bei": true, "bao": true, "ban": true, "ben": true, "bang": true, "beng": true, "bian": true, "biao": true, "bie": true, "bin": true, "bing": true,
		// 声母 p
		"pa": true, "po": true, "pi": true, "pu": true, "pai": true, "pei": true, "pao": true, "pou": true, "pan": true, "pen": true, "pang": true, "peng": true, "pian": true, "piao": true, "pie": true, "pin": true, "ping": true,
		// 声母 m
		"ma": true, "mo": true, "me": true, "mi": true, "mu": true, "mai": true, "mei": true, "mao": true, "mou": true, "man": true, "men": true, "mang": true, "meng": true, "mian": true, "miao": true, "mie": true, "min": true, "ming": true,
		// 声母 f
		"fa": true, "fo": true, "fu": true, "fei": true, "fao": true, "fou": true, "fan": true, "fen": true, "fang": true, "feng": true,
		// 声母 d
		"da": true, "de": true, "di": true, "du": true, "dai": true, "dei": true, "dao": true, "dou": true, "dan": true, "den": true, "dang": true, "deng": true, "dian": true, "diao": true, "die": true, "ding": true, "dong": true, "duan": true, "dui": true, "dun": true,
		// 声母 t
		"ta": true, "te": true, "ti": true, "tu": true, "tai": true, "tao": true, "tou": true, "tan": true, "tang": true, "teng": true, "tian": true, "tiao": true, "tie": true, "ting": true, "tong": true, "tuan": true, "tui": true, "tun": true,
		// 声母 n
		"na": true, "ne": true, "ni": true, "nu": true, "nai": true, "nei": true, "nao": true, "nou": true, "nan": true, "nen": true, "nang": true, "neng": true, "nian": true, "niao": true, "nie": true, "nin": true, "ning": true, "nong": true, "nuan": true,
		// 声母 l
		"la": true, "le": true, "li": true, "lu": true, "lai": true, "lei": true, "lao": true, "lou": true, "lan": true, "lang": true, "leng": true, "lian": true, "liao": true, "lie": true, "lin": true, "ling": true, "long": true, "luan": true, "lun": true, "luo": true,
		// 声母 g
		"ga": true, "ge": true, "gu": true, "gai": true, "gei": true, "gao": true, "gou": true, "gan": true, "gen": true, "gang": true, "geng": true, "gong": true, "guan": true, "gui": true, "gun": true, "guo": true,
		// 声母 k
		"ka": true, "ke": true, "ku": true, "kai": true, "kao": true, "kou": true, "kan": true, "ken": true, "kang": true, "keng": true, "kong": true, "kuan": true, "kui": true, "kun": true, "kuo": true,
		// 声母 h
		"ha": true, "he": true, "hu": true, "hai": true, "hei": true, "hao": true, "hou": true, "han": true, "hen": true, "hang": true, "heng": true, "hong": true, "huan": true, "hui": true, "hun": true, "huo": true,
		// 声母 j
		"ji": true, "ju": true, "jiu": true, "jie": true, "jia": true, "jiao": true, "jian": true, "jin": true, "jiang": true, "jing": true, "jiong": true, "juan": true, "jun": true,
		// 声母 q
		"qi": true, "qu": true, "qiu": true, "qie": true, "qia": true, "qiao": true, "qian": true, "qin": true, "qiang": true, "qing": true, "qiong": true, "quan": true, "qun": true,
		// 声母 x
		"xi": true, "xu": true, "xiu": true, "xie": true, "xia": true, "xiao": true, "xian": true, "xin": true, "xiang": true, "xing": true, "xiong": true, "xuan": true, "xun": true,
		// 声母 zh
		"zha": true, "zhe": true, "zhi": true, "zhu": true, "zhai": true, "zhao": true, "zhou": true, "zhan": true, "zhen": true, "zhang": true, "zheng": true, "zhong": true, "zhuan": true, "zhui": true, "zhun": true, "zhuo": true,
		// 声母 ch
		"cha": true, "che": true, "chi": true, "chu": true, "chai": true, "chao": true, "chou": true, "chan": true, "chen": true, "chang": true, "cheng": true, "chong": true, "chuan": true, "chui": true, "chun": true, "chuo": true,
		// 声母 sh
		"sha": true, "she": true, "shi": true, "shu": true, "shai": true, "shao": true, "shou": true, "shan": true, "shen": true, "shang": true, "sheng": true, "shuai": true, "shuan": true, "shui": true, "shun": true, "shuo": true,
		// 声母 r
		"ran": true, "ren": true, "rang": true, "reng": true, "ri": true, "rao": true, "rou": true, "rong": true, "ruan": true, "rui": true, "run": true, "ruo": true,
		// 声母 z
		"za": true, "ze": true, "zi": true, "zu": true, "zai": true, "zao": true, "zou": true, "zan": true, "zen": true, "zang": true, "zeng": true, "zong": true, "zuan": true, "zui": true, "zun": true, "zuo": true,
		// 声母 c
		"ca": true, "ce": true, "ci": true, "cu": true, "cai": true, "cao": true, "cou": true, "can": true, "cen": true, "cang": true, "ceng": true, "cong": true, "cuan": true, "cui": true, "cun": true, "cuo": true,
		// 声母 s
		"sa": true, "se": true, "si": true, "su": true, "sai": true, "sao": true, "sou": true, "san": true, "sen": true, "sang": true, "seng": true, "song": true, "suan": true, "sui": true, "sun": true, "suo": true,
		// 声母 y
		"ya": true, "ye": true, "yi": true, "yu": true, "yao": true, "you": true, "yan": true, "yin": true, "yang": true, "ying": true, "yong": true, "yuan": true, "yun": true, "yue": true,
		// 声母 w
		"wa": true, "wo": true, "wu": true, "wai": true, "wei": true, "wan": true, "wen": true, "wang": true, "weng": true,
		// 零声母
		"a": true, "o": true, "e": true, "ai": true, "ei": true, "ao": true, "ou": true, "an": true, "en": true, "ang": true, "eng": true, "er": true,

		// 常见的中文网站拼音
		"taobao": true, "baidu": true, "weixin": true, "zhihu": true, "youku": true, "tudou": true, "alibaba": true, "alipay": true,
		"tencent": true, "douyin": true, "weibo": true, "xiami": true, "huawei": true, "xiaomi": true, "pinduoduo": true,
		"meituan": true, "dianping": true, "ctrip": true, "feiniu": true, "suning": true, "guomei": true,
		"dangdang": true, "tianmao": true, "feishu": true,
	}

	return validPinyins[pinyin]
}

// isPinyinInitial 检查字母是否是有效的拼音声母
func isPinyinInitial(letter string) bool {
	// 所有可能的拼音声母
	initials := map[string]bool{
		"b": true, "p": true, "m": true, "f": true,
		"d": true, "t": true, "n": true, "l": true,
		"g": true, "k": true, "h": true,
		"j": true, "q": true, "x": true,
		"zh": true, "ch": true, "sh": true, "r": true,
		"z": true, "c": true, "s": true, "y": true, "w": true,
	}
	return initials[letter]
}

// isAllLetters 检查字符串是否全部由字母组成
func isAllLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
} 