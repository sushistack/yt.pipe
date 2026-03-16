package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitNarrationSentences_Basic(t *testing.T) {
	result := SplitNarrationSentences("첫 문장이다. 두 번째 문장이다.")
	assert.Equal(t, []string{"첫 문장이다.", "두 번째 문장이다."}, result)
}

func TestSplitNarrationSentences_QuotedText(t *testing.T) {
	result := SplitNarrationSentences(`그는 "가지 마라." 라고 말했다.`)
	assert.Equal(t, []string{`그는 "가지 마라." 라고 말했다.`}, result)
}

func TestSplitNarrationSentences_Ellipsis(t *testing.T) {
	result := SplitNarrationSentences("그것은... 무언가였다.")
	assert.Equal(t, []string{"그것은... 무언가였다."}, result)
}

func TestSplitNarrationSentences_Single(t *testing.T) {
	result := SplitNarrationSentences("하나의 문장이다.")
	assert.Equal(t, []string{"하나의 문장이다."}, result)
}

func TestSplitNarrationSentences_Empty(t *testing.T) {
	assert.Nil(t, SplitNarrationSentences(""))
	assert.Nil(t, SplitNarrationSentences("   "))
}

func TestSplitNarrationSentences_KoreanEndings(t *testing.T) {
	result := SplitNarrationSentences("SCP-173은 콘크리트 조각상이다. 눈을 떼면 움직인다. 매우 위험하다.")
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "SCP-173은 콘크리트 조각상이다.", result[0])
	assert.Equal(t, "눈을 떼면 움직인다.", result[1])
	assert.Equal(t, "매우 위험하다.", result[2])
}

func TestSplitNarrationSentences_QuestionMark(t *testing.T) {
	result := SplitNarrationSentences("무엇일까? 아무도 모른다.")
	assert.Equal(t, []string{"무엇일까?", "아무도 모른다."}, result)
}

func TestSplitNarrationSentences_NoTrailingPunctuation(t *testing.T) {
	result := SplitNarrationSentences("첫 번째다. 마지막 문장")
	assert.Equal(t, []string{"첫 번째다.", "마지막 문장"}, result)
}
