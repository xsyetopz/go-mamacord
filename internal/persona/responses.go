package persona

import (
	"encoding/binary"
	"hash/fnv"
	"strings"
)

const (
	mommyDefault = "mommy"

	mommyMummy  = "mummy"
	mommyMama   = "mama"
	mommyMamma  = "mamma"
	mommyMami   = "mami"
	mommyGerman = "Mama"
	mommyFrench = "Maman"
	mommyCzech  = "máma"
	mommyFinn   = "äiti"
	mommyViet   = "mẹ"
	mommyLith   = "mamytė"
	mommyCro    = "mama"
	mommyHindi  = "मम्मी"
	mommyHung   = "anya"
	mommyGreek  = "μανούλα"
	mommyPTBR   = "mamãe"
	mommyThai   = "มามี้"
	mommyTurk   = "annecik"

	mommySpanishES = "mamá"
	mommyCyrillic  = "мама"

	mommyJapanese = "ママ"
	mommyKorean   = "엄마"
	mommyZHCN     = "妈妈"
	mommyZHTW     = "媽咪"
)

func Mommy(locale string) string {
	// Keep the persona fixed, but allow minor locale-adjacent wording.
	code := strings.ToLower(strings.TrimSpace(locale))
	mommyByLocale := map[string]string{
		"bg":     mommyCyrillic,
		"cs":     mommyCzech,
		"da":     mommyMamma,
		"de":     mommyGerman,
		"el":     mommyGreek,
		"en-gb":  mommyMummy,
		"es-419": mommyMami,
		"es-es":  mommySpanishES,
		"fi":     mommyFinn,
		"fr":     mommyFrench,
		"hi":     mommyHindi,
		"hr":     mommyCro,
		"hu":     mommyHung,
		"id":     mommyMama,
		"it":     mommyMamma,
		"ja":     mommyJapanese,
		"ko":     mommyKorean,
		"lt":     mommyLith,
		"nl":     mommyMama,
		"no":     mommyMamma,
		"pl":     mommyMama,
		"pt-br":  mommyPTBR,
		"ro":     mommyMama,
		"ru":     mommyCyrillic,
		"sv-se":  mommyMamma,
		"th":     mommyThai,
		"tr":     mommyTurk,
		"uk":     mommyCyrillic,
		"vi":     mommyViet,
		"zh-cn":  mommyZHCN,
		"zh-tw":  mommyZHTW,
	}
	if v, ok := mommyByLocale[code]; ok {
		return v
	}
	return mommyDefault
}

func PetName(locale string, userID uint64, messageID string) string {
	terms := petTerms(locale)
	if len(terms) == 0 {
		return "kiddo"
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.TrimSpace(locale)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.TrimSpace(messageID)))
	_, _ = h.Write([]byte{0})

	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], userID)
	_, _ = h.Write(b[:])

	mod := h.Sum64() % uint64(len(terms))
	maxInt := uint64(^uint(0) >> 1)
	if mod > maxInt {
		mod = 0
	}
	return terms[int(mod)]
}

func petTerms(locale string) []string {
	code := strings.ToLower(strings.TrimSpace(locale))
	if code == "" {
		return defaultPetTerms()
	}

	if terms := petTermsSpanish(code); len(terms) > 0 {
		return terms
	}
	if terms := petTermsEurope(code); len(terms) > 0 {
		return terms
	}
	if terms := petTermsCyrillic(code); len(terms) > 0 {
		return terms
	}
	if terms := petTermsAsia(code); len(terms) > 0 {
		return terms
	}

	return defaultPetTerms()
}

func defaultPetTerms() []string {
	return []string{"kiddo", "sweetie", "little one", "honey", "pumpkin", "sunshine", "love", "dear"}
}

func petTermsSpanish(code string) []string {
	switch code {
	case "es-es":
		return []string{"cariño", "cielo", "tesoro", "corazón", "peque", "amor"}
	case "es-419":
		return []string{"cariño", "cielito", "tesoro", "corazón", "chiqui", "mi amor"}
	default:
		return nil
	}
}

func petTermsEurope(code string) []string {
	switch code {
	case "da":
		return []string{"skat", "søde", "lille ven"}
	case "de":
		return []string{"liebling", "schatz", "kleines"}
	case "fi":
		return []string{"kulta", "rakas", "pikkuinen", "muru", "sydänkäpynen"}
	case "fr":
		return []string{"mon cœur", "petit cœur", "mon trésor"}
	case "hr":
		return []string{"srce", "dušo", "mali"}
	case "hu":
		return []string{"drágám", "kicsim", "édesem"}
	case "it":
		return []string{"tesoro", "cuoricino", "piccolino"}
	case "lt":
		return []string{"mielas", "širdelė", "mažyli"}
	case "nl":
		return []string{"lieverd", "schat", "kleintje"}
	case "no":
		return []string{"kjære", "skatt", "lille venn"}
	case "pl":
		return []string{"kochanie", "skarbie", "słonko"}
	case "pt-br":
		return []string{"querido", "docinho", "meu bem"}
	case "ro":
		return []string{"pui", "suflet", "drag"}
	case "sv-se":
		return []string{"älskling", "vännen", "lilla vän"}
	case "cs":
		return []string{"zlatíčko", "sluníčko", "drobku"}
	case "el":
		return []string{"καρδιά μου", "αγάπη μου", "μικρό μου"}
	case "tr":
		return []string{"canım", "tatlım", "küçüğüm"}
	default:
		return nil
	}
}

func petTermsCyrillic(code string) []string {
	switch code {
	case "bg":
		return []string{"слънчице", "сърчице", "миличко"}
	case "ru":
		return []string{"солнышко", "зайка", "котик"}
	case "uk":
		return []string{"сонечко", "зайченя", "котик"}
	default:
		return nil
	}
}

func petTermsAsia(code string) []string {
	switch code {
	case "id":
		return []string{"sayang", "manis", "teman kecil"}
	case "hi":
		return []string{"बच्चे", "प्यारे", "नन्हे"}
	case "th":
		return []string{"ที่รัก", "คนเก่ง", "เด็กน้อย"}
	case "vi":
		return []string{"bé", "cưng", "nhỏ"}
	case "zh-cn":
		return []string{"宝贝", "小可爱", "乖乖"}
	case "zh-tw":
		return []string{"寶貝", "小可愛", "乖乖"}
	case "ja":
		return []string{"いい子", "ハニー", "かわいい子"}
	case "ko":
		return []string{"아가", "꼬마야", "귀염둥이"}
	default:
		return nil
	}
}
