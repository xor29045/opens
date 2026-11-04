#!/usr/bin/env bash
# Creates a house (manager account + establishment page) and its
# professionals through the public API, with the usernames and passwords
# written below. Re-running the script is safe: existing accounts are reused
# and existing professionals are skipped.
#
# Usage: scripts/create-house.sh [API_URL]
#   API_URL defaults to https://localhost:8443/api/v1 (local docker compose).
# Requires: curl, jq.
set -euo pipefail

API="${1:-${API_URL:-https://localhost:8443/api/v1}}"

# ---------------------------------------------------------------- settings
# Manager (12 characters minimum for every password).
MANAGER_USERNAME="maison_rose"
MANAGER_PASSWORD="motdepasse-gerant-2026"

# Establishment page.
HOUSE_NAME="Maison Rose"
HOUSE_LEGAL_NAME="Maison Rose SAS"
HOUSE_DESCRIPTION="Une maison chaleureuse au pied de Montmartre. Accueil discret et équipe attentive."
HOUSE_DEPARTEMENT="75-paris"          # slug of the département (GET /regions)
HOUSE_ADDRESS="12 rue des Martyrs, 75009 Paris"

# Platform admin, used to activate the house subscription without payment
# (POST /admin/establishments/{id}/subscription/activate). Leave the password
# empty to skip the activation. The admin must have changed its bootstrap
# password once in the admin interface.
ADMIN_USERNAME="opensex_admin"
ADMIN_PASSWORD="hbnFUs57N3Pzk5aar062qF"
SUBSCRIPTION_DAYS=30

# Professionals with their full profile. Prices are in roses (1 rose = 1 €).
# Vocabularies (GET /api/v1/attributes): hair_color BLONDE BROWN BLACK RED GREY
# OTHER · eye_color BLUE GREEN BROWN HAZEL GREY BLACK OTHER · body_type SLIM
# ATHLETIC AVERAGE CURVY PLUS_SIZE MUSCULAR · skin_tone FAIR LIGHT MEDIUM OLIVE
# TAN BROWN DARK · gender FEMALE MALE TRANS_WOMAN TRANS_MAN NON_BINARY.
# Categories are slugs of GET /api/v1/categories. Contact visibility: PUBLIC
# or HIDDEN.
PROFESSIONALS=$(cat <<'JSON'
[
  {
    "display_name": "Lola", "username": "lola_rose", "password": "motdepasse-lola-2026",
    "headline": "Douce, souriante et attentionnée",
    "description": "Lola aime prendre le temps : une conversation autour d'un verre, une ambiance tamisée et beaucoup de douceur. Elle accueille au salon Montmartre de la maison, dans un cadre discret et soigné.",
    "age": 26, "height_cm": 168, "weight_kg": 56, "hair_color": "BLONDE", "eye_color": "BLUE", "body_type": "SLIM",
    "skin_tone": "FAIR", "gender": "FEMALE", "languages": ["fr", "en"],
    "categories": ["massage", "accompagnement", "sur-place"],
    "rates": [{"label": "30 minutes", "minutes": 30, "roses": 100}, {"label": "1 heure", "minutes": 60, "roses": 180},
              {"label": "Soirée", "minutes": 180, "roses": 450}],
    "contacts": [{"type": "PHONE", "value": "+33 6 12 34 56 01", "visibility": "PUBLIC"},
                 {"type": "WHATSAPP", "value": "+33 6 12 34 56 01", "visibility": "PUBLIC"}]
  },
  {
    "display_name": "Mia", "username": "mia_rose", "password": "motdepasse-mia-2026",
    "headline": "Pétillante, curieuse et pleine d'humour",
    "description": "Mia a le rire facile et l'esprit vif. Passionnée de danse, elle propose des moments légers et complices, à la maison ou en accompagnement pour un dîner.",
    "age": 31, "height_cm": 172, "weight_kg": 60, "hair_color": "BROWN", "eye_color": "HAZEL", "body_type": "ATHLETIC",
    "skin_tone": "OLIVE", "gender": "FEMALE", "languages": ["fr", "es", "en"],
    "categories": ["accompagnement", "jeux-de-role", "deplacement"],
    "rates": [{"label": "1 heure", "minutes": 60, "roses": 200}, {"label": "2 heures", "minutes": 120, "roses": 360},
              {"label": "Dîner et soirée", "minutes": 240, "roses": 650}],
    "contacts": [{"type": "PHONE", "value": "+33 6 12 34 56 02", "visibility": "PUBLIC"},
                 {"type": "TELEGRAM", "value": "@mia_rose_paris", "visibility": "PUBLIC"}]
  },
  {
    "display_name": "Nina", "username": "nina_rose", "password": "motdepasse-nina-2026",
    "headline": "Élégante et raffinée, l'art du massage",
    "description": "Formée au massage californien et tantrique, Nina propose des séances relaxantes dans une salle dédiée de la maison : huiles chaudes, musique douce et lumière tamisée.",
    "age": 35, "height_cm": 165, "weight_kg": 58, "hair_color": "BLACK", "eye_color": "BROWN", "body_type": "CURVY",
    "skin_tone": "BROWN", "gender": "FEMALE", "languages": ["fr", "en", "ar"],
    "categories": ["massage", "massage-tantrique", "sur-place"],
    "rates": [{"label": "Massage 45 minutes", "minutes": 45, "roses": 150}, {"label": "Massage 1 h 30", "minutes": 90, "roses": 260}],
    "contacts": [{"type": "PHONE", "value": "+33 6 12 34 56 03", "visibility": "PUBLIC"},
                 {"type": "INSTAGRAM", "value": "https://instagram.com/nina.rose.massage", "visibility": "PUBLIC"}]
  },
  {
    "display_name": "Jade", "username": "jade_rose", "password": "motdepasse-jade-2026",
    "headline": "Caractère affirmé, univers sur mesure",
    "description": "Jade aime les rencontres qui sortent de l'ordinaire. Elle prend le temps d'échanger sur vos envies avant chaque rendez-vous, toujours dans le respect et la bienveillance.",
    "age": 29, "height_cm": 178, "weight_kg": 63, "hair_color": "RED", "eye_color": "GREEN", "body_type": "ATHLETIC",
    "skin_tone": "LIGHT", "gender": "FEMALE", "languages": ["fr", "en", "it"],
    "categories": ["domination", "fetichisme", "jeux-de-role", "sur-place"],
    "rates": [{"label": "1 heure", "minutes": 60, "roses": 250}, {"label": "2 heures", "minutes": 120, "roses": 450}],
    "contacts": [{"type": "PHONE", "value": "+33 6 12 34 56 04", "visibility": "PUBLIC"}]
  },
  {
    "display_name": "Sasha", "username": "sasha_rose", "password": "motdepasse-sasha-2026",
    "headline": "Lumineuse et libre, rencontres en toute simplicité",
    "description": "Sasha accueille avec naturel et beaucoup d'écoute. Couples bienvenus : elle aime les moments partagés, sans jugement et dans la bonne humeur.",
    "age": 24, "height_cm": 175, "weight_kg": 59, "hair_color": "BLONDE", "eye_color": "GREY", "body_type": "SLIM",
    "skin_tone": "MEDIUM", "gender": "TRANS_WOMAN", "languages": ["fr", "en", "ru"],
    "categories": ["couples", "duo", "accompagnement", "sur-place"],
    "rates": [{"label": "1 heure", "minutes": 60, "roses": 200}, {"label": "Couple 1 heure", "minutes": 60, "roses": 300}],
    "contacts": [{"type": "PHONE", "value": "+33 6 12 34 56 05", "visibility": "PUBLIC"},
                 {"type": "WHATSAPP", "value": "+33 6 12 34 56 05", "visibility": "PUBLIC"}]
  },
  {
    "display_name": "Leo", "username": "leo_rose", "password": "motdepasse-leo-2026",
    "headline": "Charmant, attentif et grand sportif",
    "description": "Leo propose des massages et des moments de détente pour une clientèle féminine et les couples. Coach sportif le jour, il est réputé pour son calme et sa délicatesse.",
    "age": 33, "height_cm": 186, "weight_kg": 82, "hair_color": "BROWN", "eye_color": "BROWN", "body_type": "MUSCULAR",
    "skin_tone": "TAN", "gender": "MALE", "languages": ["fr", "en", "pt"],
    "categories": ["massage", "couples", "deplacement"],
    "rates": [{"label": "Massage 1 heure", "minutes": 60, "roses": 180}, {"label": "2 heures", "minutes": 120, "roses": 320}],
    "contacts": [{"type": "PHONE", "value": "+33 6 12 34 56 06", "visibility": "PUBLIC"}]
  }
]
JSON
)

# Optional photos: PHOTOS_DIR/<username>*.jpg (or .png, .webp) are uploaded,
# the first one becomes the main photo. Without photos the profile shows a
# silhouette.
PHOTOS_DIR="$(dirname "$0")/photos"
# ------------------------------------------------------------------------

for bin in curl jq; do
  command -v "$bin" >/dev/null || { echo "error: $bin is required" >&2; exit 1; }
done

# -k: the local stack uses Caddy's internal certificate.
CURL=(curl -sk --max-time 20 -H "Content-Type: application/json")
STATUS=""
BODY=""

# call METHOD PATH [JSON] [TOKEN] — sets STATUS and BODY.
call() {
  local method=$1 path=$2 data=${3:-} token=${4:-} out
  local args=("${CURL[@]}" -X "$method" -w $'\n%{http_code}' "$API$path")
  [[ -n $data ]] && args+=(--data "$data")
  [[ -n $token ]] && args+=(-H "Authorization: Bearer $token")
  out=$("${args[@]}") || { echo "error: cannot reach $API" >&2; exit 1; }
  STATUS=${out##*$'\n'}
  BODY=${out%$'\n'*}
}

fail() {
  echo "error: $1 (HTTP $STATUS)" >&2
  echo "$BODY" | jq . 2>/dev/null >&2 || echo "$BODY" >&2
  exit 1
}

# 1. Manager account: register, or log in if it already exists.
creds=$(jq -n --arg u "$MANAGER_USERNAME" --arg p "$MANAGER_PASSWORD" '{username: $u, password: $p}')
call POST /auth/register/manager "$creds"
if [[ $STATUS == 201 ]]; then
  echo "manager $MANAGER_USERNAME created"
elif [[ $STATUS == 409 ]]; then
  call POST /auth/login "$creds"
  [[ $STATUS == 200 ]] || fail "manager $MANAGER_USERNAME exists but the password does not match"
  echo "manager $MANAGER_USERNAME already exists, logged in"
else
  fail "cannot create the manager"
fi
TOKEN=$(jq -r .access_token <<<"$BODY")

# 2. Establishment page, located in the chosen département.
call GET /regions
[[ $STATUS == 200 ]] || fail "cannot list the départements"
city_id=$(jq -r --arg s "$HOUSE_DEPARTEMENT" '.items[] | select(.slug == $s) | .id' <<<"$BODY")
region_id=$(jq -r --arg s "$HOUSE_DEPARTEMENT" '(.items[] | select(.slug == $s) | .parent_id) // empty' <<<"$BODY")
[[ -n $city_id && -n $region_id ]] || { echo "error: unknown département $HOUSE_DEPARTEMENT" >&2; exit 1; }

page=$(jq -n --arg n "$HOUSE_NAME" --arg l "$HOUSE_LEGAL_NAME" --arg d "$HOUSE_DESCRIPTION" \
  --arg r "$region_id" --arg c "$city_id" --arg a "$HOUSE_ADDRESS" \
  '{display_name: $n, legal_name: $l, description: $d, region_id: $r, city_id: $c, address: $a, address_public: false,
    opening_hours: {mon: [{open: "14:00", close: "02:00"}], tue: [{open: "14:00", close: "02:00"}],
                    wed: [{open: "14:00", close: "02:00"}], thu: [{open: "14:00", close: "02:00"}],
                    fri: [{open: "14:00", close: "04:00"}], sat: [{open: "14:00", close: "04:00"}]}}')
call PUT /manager/establishment "$page" "$TOKEN"
[[ $STATUS == 200 ]] || fail "cannot fill the establishment page"
ESTABLISHMENT_ID=$(jq -r .id <<<"$BODY")
echo "establishment \"$HOUSE_NAME\" ready ($HOUSE_DEPARTEMENT)"

# 3. Professionals: account, profile, categories, rates, contacts, photos.
call GET /categories
[[ $STATUS == 200 ]] || fail "cannot list the categories"
CATEGORIES=$BODY

profile_id_of() { # USERNAME — id of the house profile linked to this login
  call GET "/manager/profiles?page_size=100" "" "$TOKEN"
  [[ $STATUS == 200 ]] || fail "cannot list the profiles"
  jq -r --arg u "$1" '.items[] | select(.linked_account.username == $u) | .id' <<<"$BODY"
}

count=$(jq length <<<"$PROFESSIONALS")
for ((i = 0; i < count; i++)); do
  pro=$(jq ".[$i]" <<<"$PROFESSIONALS")
  name=$(jq -r .display_name <<<"$pro")
  username=$(jq -r .username <<<"$pro")

  call POST /manager/profiles "$(jq '{display_name, username, password}' <<<"$pro")" "$TOKEN"
  case $STATUS in
    201) id=$(jq -r .id <<<"$BODY"); echo "professional $name created (login: $username)" ;;
    409)
      id=$(profile_id_of "$username")
      [[ -n $id ]] || { echo "error: username $username is taken by an account outside this house" >&2; exit 1; }
      echo "professional $name already exists, profile updated" ;;
    *) fail "cannot create the professional $name" ;;
  esac
  base="/manager/profiles/$id"

  call PUT "$base" "$(jq '{display_name, headline, description, age, height_cm, weight_kg, hair_color, eye_color, body_type,
    languages, extra_attributes: {gender, skin_tone}}' <<<"$pro")" "$TOKEN"
  [[ $STATUS == 200 ]] || fail "cannot fill the profile of $name"

  ids=$(jq -c --argjson cats "$CATEGORIES" '[.categories[] as $s | $cats.items[] | select(.slug == $s) | .id]' <<<"$pro")
  call PUT "$base/categories" "{\"category_ids\": $ids}" "$TOKEN"
  [[ $STATUS == 200 ]] || fail "cannot set the categories of $name"

  call PUT "$base/rates" "$(jq '{rates: [.rates[] | {label: .label, duration_minutes: .minutes, amount_cents: (.roses * 100)}]}' <<<"$pro")" "$TOKEN"
  [[ $STATUS == 200 ]] || fail "cannot set the rates of $name"

  call PUT "$base/contacts" "$(jq '{contacts}' <<<"$pro")" "$TOKEN"
  [[ $STATUS == 200 ]] || fail "cannot set the contacts of $name"

  # Photos only once: a profile that already has some is left as is.
  call GET "$base" "" "$TOKEN"
  if [[ $(jq '.photos | length' <<<"$BODY") == 0 ]]; then
    shopt -s nullglob
    for photo in "$PHOTOS_DIR/$username"*.{jpg,jpeg,png,webp}; do
      code=$(curl -sk --max-time 60 -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOKEN" \
        -F "file=@$photo" "$API$base/photos")
      [[ $code == 201 ]] && echo "  photo $(basename "$photo") added" || echo "  photo $(basename "$photo") refused (HTTP $code)" >&2
    done
    shopt -u nullglob
  fi
done

# 4. Active subscription, granted by the platform admin: the profiles go in
#    service at once.
SUBSCRIPTION="not activated (ADMIN_PASSWORD is empty)"
if [[ -n $ADMIN_PASSWORD ]]; then
  admin=$(jq -n --arg u "$ADMIN_USERNAME" --arg p "$ADMIN_PASSWORD" '{username: $u, password: $p}')
  call POST /auth/admin/login "$admin"
  [[ $STATUS == 200 ]] || fail "admin login refused"
  if [[ $(jq -r .user.must_change_password <<<"$BODY") == true ]]; then
    echo "error: the admin must change its bootstrap password first (/admin/login)" >&2
    exit 1
  fi
  ADMIN_TOKEN=$(jq -r .access_token <<<"$BODY")
  call POST "/admin/establishments/$ESTABLISHMENT_ID/subscription/activate" "{\"days\": $SUBSCRIPTION_DAYS}" "$ADMIN_TOKEN"
  [[ $STATUS == 200 ]] || fail "cannot activate the subscription"
  SUBSCRIPTION="active until $(jq -r .current_period_end <<<"$BODY")"
  echo "subscription $SUBSCRIPTION"
fi

cat <<EOF

Done.
  Manager:       $MANAGER_USERNAME / $MANAGER_PASSWORD  (/gerant/login)
  Professionals: log in on /pro/login with the usernames and passwords of the script.
  Subscription:  $SUBSCRIPTION
Profiles are in service and published once the subscription is active.
EOF
