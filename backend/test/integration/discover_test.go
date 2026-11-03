package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func deckIDs(e *env, token string) ([]string, float64) {
	r := e.get("/api/v1/client/discover", token).expect(http.StatusOK).JSON()
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

func favoriteIDs(e *env, token string) map[string]bool {
	out := map[string]bool{}
	for _, it := range e.get("/api/v1/client/favorites", token).expect(http.StatusOK).JSON()["items"].([]any) {
		out[it.(map[string]any)["profile_id"].(string)] = true
	}
	return out
}

// Like / dislike deck of the client space: likes become favorites, dislikes
// hide the profile; both can be undone; each client has its own deck.
func TestDiscoverDeck(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m, slug := e.readyManager()
	_, id1 := e.publishedProfile(m, slug)
	_, id2 := e.publishedProfile(m, slug)
	a := e.register("client", nil)
	b := e.register("client", nil)

	ids, remaining := deckIDs(e, a.Token)
	if len(ids) != 2 || remaining != 2 {
		t.Fatalf("deck %v remaining %v", ids, remaining)
	}
	card := e.get("/api/v1/client/discover", a.Token).JSON()["items"].([]any)[0].(map[string]any)
	if len(card["photos"].([]any)) == 0 || card["age"] == nil {
		t.Fatalf("card incomplete: %v", card)
	}

	// Like → favorite, out of the deck.
	e.put("/api/v1/client/favorites/"+id1, a.Token, nil).expect(http.StatusNoContent)
	if ids, _ := deckIDs(e, a.Token); len(ids) != 1 || ids[0] != id2 {
		t.Fatalf("liked profile still in deck: %v", ids)
	}
	if !favoriteIDs(e, a.Token)[id1] {
		t.Fatal("liked profile must be a favorite")
	}

	// Dislike → hidden; undo → back.
	e.put("/api/v1/client/dislikes/"+id2, a.Token, nil).expect(http.StatusNoContent)
	if ids, remaining := deckIDs(e, a.Token); len(ids) != 0 || remaining != 0 {
		t.Fatalf("disliked profile still in deck: %v", ids)
	}
	e.del("/api/v1/client/dislikes/"+id2, a.Token, nil).expect(http.StatusNoContent)
	if ids, _ := deckIDs(e, a.Token); len(ids) != 1 {
		t.Fatal("undo should bring the profile back")
	}
	e.del("/api/v1/client/dislikes/"+id2, a.Token, nil).expect(http.StatusNotFound)

	// Disliking a favorite removes it from the favorites.
	e.put("/api/v1/client/dislikes/"+id1, a.Token, nil).expect(http.StatusNoContent)
	if favoriteIDs(e, a.Token)[id1] {
		t.Fatal("a disliked profile must leave the favorites")
	}

	// Reset brings every passed profile back.
	r := e.del("/api/v1/client/dislikes", a.Token, nil).expect(http.StatusOK).JSON()
	if r["restored"].(float64) != 1 {
		t.Fatalf("restored %v", r["restored"])
	}

	// Other clients are not affected; unknown or unpublished profiles → 404.
	if ids, _ := deckIDs(e, b.Token); len(ids) != 2 {
		t.Fatal("client B must see the whole deck")
	}
	e.put("/api/v1/client/dislikes/"+uuid.NewString(), a.Token, nil).expect(http.StatusNotFound)
	e.post("/api/v1/admin/profiles/"+id2+"/hide", adm.Token, map[string]any{"reason": "contrôle"}).expect(http.StatusNoContent)
	e.put("/api/v1/client/dislikes/"+id2, b.Token, nil).expect(http.StatusNotFound)
	if ids, _ := deckIDs(e, b.Token); len(ids) != 1 || ids[0] != id1 {
		t.Fatalf("hidden profile must leave the deck: %v", ids)
	}

	// Only clients have a deck.
	e.get("/api/v1/client/discover", m.Token).expect(http.StatusForbidden)
}
