package main

type Category int

const (
	Politic Category = iota + 1

	Economy
	Social
	LivingCulture
	ItScience
	Global
)

func (c Category) String() string {
	switch c {
	case Politic:
		return "Politic"
	case Economy:
		return "Economy"
	case Social:
		return "Social"
	case LivingCulture:
		return "LivingCulture"
	case ItScience:
		return "ItScience"
	case Global:
		return "Global"
	default:
		return "Invalid Category"
	}
}

func (c Category) Url() string {
	switch c {
	case Politic:
		return "https://news.naver.com/section/100"
	case Economy:
		return "https://news.naver.com/section/101"
	case Social:
		return "https://news.naver.com/section/102"
	case LivingCulture:
		return "https://news.naver.com/section/103"
	case ItScience:
		return "https://news.naver.com/section/105"
	case Global:
		return "https://news.naver.com/section/104"
	default:
		return "Invalid Category"
	}
}
