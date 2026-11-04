package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func deck(e *env, exclude ...string) ([]string, float64) {
	if exclude == nil {
		exclude = []string{}
	}
	r := e.post("/api/v1/discover", "", map[string]any{"exclude": exclude}).expect(http.StatusOK).JSON()
	var ids []string
	for _, it := range r["items"].([]any) {
		card := it.(map[string]any)
		if _, leaked := card["contacts"]; leaked {
			e.t.Fatal("the deck must not expose contacts")
		}
		ids = append(ids, card["id"].(string))
	}
	return ids, r["remaining"].(float64)
}

// ADR-023: the "Découvrir" deck is anonymous. Likes and passes live in the
// browser; the server leaves out the ids it is given and describes the
// liked profiles that are still published.
func TestDiscoverDeck(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m, slug := e.readyManager()
	_, id1 := e.publishedProfile(m, slug)
	_, id2 := e.publishedProfile(m, slug)

	ids, remaining := deck(e)
	if len(ids) != 2 || remaining != 2 {
		t.Fatalf("deck %v remaining %v", ids, remaining)
	}
	card := e.post("/api/v1/discover", "", map[string]any{"limit": 1}).expect(200).JSON()["items"].([]any)[0].(map[string]any)
	if len(card["photos"].([]any)) == 0 {
		t.Fatalf("card incomplete: %v", card)
	}
	if ids, remaining := deck(e, id1); len(ids) != 1 || ids[0] != id2 || remaining != 1 {
		t.Fatalf("excluded profile still in deck: %v", ids)
	}
	if ids, _ := deck(e, id1, id2, uuid.NewString()); len(ids) != 0 {
		t.Fatalf("deck should be empty: %v", ids)
	}

	// Favorites page: liked profiles still published.
	cards := e.post("/api/v1/discover/cards", "", map[string]any{"ids": []string{id1, uuid.NewString()}}).expect(200).JSON()["items"].([]any)
	if len(cards) != 1 || cards[0].(map[string]any)["id"] != id1 {
		t.Fatalf("cards %v", cards)
	}
	e.post("/api/v1/admin/profiles/"+id2+"/hide", adm.Token, map[string]any{"reason": "contrôle"}).expect(http.StatusNoContent)
	if ids, _ := deck(e); len(ids) != 1 || ids[0] != id1 {
		t.Fatalf("hidden profile must leave the deck: %v", ids)
	}
	if n := len(e.post("/api/v1/discover/cards", "", map[string]any{"ids": []string{id2}}).expect(200).JSON()["items"].([]any)); n != 0 {
		t.Fatal("hidden profile must not be described")
	}

	// Malformed ids and oversized lists are refused.
	e.post("/api/v1/discover", "", map[string]any{"exclude": []string{"nope"}}).expect(http.StatusBadRequest)
	many := make([]string, 201)
	for i := range many {
		many[i] = uuid.NewString()
	}
	e.post("/api/v1/discover/cards", "", map[string]any{"ids": many}).expect(http.StatusUnprocessableEntity)
}

// ADR-023: client accounts no longer exist.
func TestNoClientAccounts(t *testing.T) {
	e := newEnv(t)
	e.post("/api/v1/auth/register/client", "", map[string]any{"username": "client_x1", "password": pwd, "accept_terms": true}).
		expect(http.StatusNotFound)
	e.get("/api/v1/client/favorites", "").expect(http.StatusNotFound)
}

// The deck filters: city, hair, eyes, skin, height, age, price, gender.
func TestDiscoverFilters(t *testing.T) {
	e := newEnv(t)
	m, slug := e.readyManager()
	_, a := e.publishedProfile(m, slug)
	_, b := e.publishedProfile(m, slug)
	e.put("/api/v1/manager/profiles/"+a, m.Token, map[string]any{"display_name": "Lola", "age": 25, "height_cm": 165,
		"hair_color": "BLONDE", "eye_color": "BLUE", "extra_attributes": map[string]any{"gender": "FEMALE", "skin_tone": "FAIR"}}).expect(200)
	e.put("/api/v1/manager/profiles/"+b, m.Token, map[string]any{"display_name": "Leo", "age": 40, "height_cm": 185,
		"hair_color": "BLACK", "eye_color": "BROWN", "extra_attributes": map[string]any{"gender": "MALE", "skin_tone": "DARK"}}).expect(200)
	e.put("/api/v1/manager/profiles/"+a, m.Token, map[string]any{"display_name": "x", "age": 17}).expect(422)

	only := func(filters map[string]any) []string {
		t.Helper()
		r := e.post("/api/v1/discover", "", map[string]any{"filters": filters}).expect(http.StatusOK).JSON()
		var ids []string
		for _, it := range r["items"].([]any) {
			ids = append(ids, it.(map[string]any)["id"].(string))
		}
		return ids
	}
	for name, tc := range map[string]struct {
		filters map[string]any
		want    string
	}{
		"hair":   {map[string]any{"hair_color": []string{"BLONDE"}}, a},
		"eyes":   {map[string]any{"eye_color": []string{"BROWN"}}, b},
		"skin":   {map[string]any{"skin_tone": []string{"FAIR", "LIGHT"}}, a},
		"gender": {map[string]any{"gender": []string{"MALE"}}, b},
		"height": {map[string]any{"height_min": 180}, b},
		"age":    {map[string]any{"age_min": 18, "age_max": 30}, a},
	} {
		if ids := only(tc.filters); len(ids) != 1 || ids[0] != tc.want {
			t.Errorf("%s: %v", name, ids)
		}
	}
	if ids := only(map[string]any{"city": "75-paris", "price_max": 200}); len(ids) != 2 {
		t.Errorf("city+price: %v", ids)
	}
	if ids := only(map[string]any{"city": "13-bouches-du-rhone"}); len(ids) != 0 {
		t.Errorf("other city: %v", ids)
	}
	if ids := only(map[string]any{"price_max": 100}); len(ids) != 0 {
		t.Errorf("price: %v", ids)
	}
	card := e.post("/api/v1/discover", "", map[string]any{"filters": map[string]any{"hair_color": []string{"BLONDE"}}}).
		expect(200).JSON()["items"].([]any)[0].(map[string]any)
	if card["age"].(float64) != 25 {
		t.Errorf("card age %v", card["age"])
	}
	e.post("/api/v1/discover", "", map[string]any{"filters": map[string]any{"hair_color": []string{"PINK"}}}).expect(422)
	e.post("/api/v1/discover", "", map[string]any{"filters": map[string]any{"age_min": 40, "age_max": 20}}).expect(422)
}
