package news

import "testing"

func TestScore_Positive(t *testing.T) {
	score := Score("Bitcoin surges to a record high milestone rally")
	if score <= 0 {
		t.Errorf("expected positive score, got %f", score)
	}
}

func TestScore_Negative(t *testing.T) {
	score := Score("Crypto market crash as prices plunge and drop amid selloff")
	if score >= 0 {
		t.Errorf("expected negative score, got %f", score)
	}
}

func TestScore_Neutral(t *testing.T) {
	score := Score("The market opened today for trading")
	if score != 0 {
		t.Errorf("expected zero score, got %f", score)
	}
}

func TestScore_Range(t *testing.T) {
	texts := []string{
		"Bull market rally surge record high milestone",
		"crash plunge fall drop bear selloff bankruptcy",
		"",
		"news article",
	}
	for _, text := range texts {
		s := Score(text)
		if s < -1 || s > 1 {
			t.Errorf("Score(%q) = %f out of [-1,1]", text, s)
		}
	}
}
