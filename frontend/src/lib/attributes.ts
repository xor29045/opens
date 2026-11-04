// Closed vocabularies of the profile attributes (server: GET /attributes)
// with their French labels. Shared by the editor, the profile page and the
// "Découvrir" filters.
type Options = [value: string, label: string][];

export const hairColors: Options = [
  ['BLONDE', 'Blonds'], ['BROWN', 'Châtains'], ['BLACK', 'Noirs'], ['RED', 'Roux'], ['GREY', 'Gris'], ['OTHER', 'Autre'],
];
export const eyeColors: Options = [
  ['BLUE', 'Bleus'], ['GREEN', 'Verts'], ['BROWN', 'Marron'], ['HAZEL', 'Noisette'], ['GREY', 'Gris'], ['BLACK', 'Noirs'], ['OTHER', 'Autre'],
];
export const skinTones: Options = [
  ['FAIR', 'Très claire'], ['LIGHT', 'Claire'], ['MEDIUM', 'Moyenne'], ['OLIVE', 'Olive'], ['TAN', 'Mate'], ['BROWN', 'Brune'], ['DARK', 'Noire'],
];
export const genders: Options = [
  ['FEMALE', 'Femme'], ['MALE', 'Homme'], ['TRANS_WOMAN', 'Femme trans'], ['TRANS_MAN', 'Homme trans'], ['NON_BINARY', 'Non binaire'],
];
export const bodyTypes: Options = [
  ['SLIM', 'Mince'], ['ATHLETIC', 'Sportive'], ['AVERAGE', 'Moyenne'], ['CURVY', 'Pulpeuse'], ['PLUS_SIZE', 'Ronde'], ['MUSCULAR', 'Musclée'],
];

export function labelOf(options: Options, value: string | null | undefined): string | null {
  if (!value) return null;
  return options.find(([v]) => v === value)?.[1] ?? value;
}
