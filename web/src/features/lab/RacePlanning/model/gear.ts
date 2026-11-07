import type { GearChecklist, GearContext, GearItem } from './types';

type GearCategory = GearItem['category'];

interface GearCandidate {
  item: GearItem;
  category: GearCategory;
}

/** Non-finite or negative numeric inputs are treated as zero. */
function nonNegative(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function makeItem(id: string, name: string, category: GearCategory, reason: string): GearCandidate {
  return { item: { id, name, category, reason }, category };
}

/**
 * Derives the mandatory and recommended gear checklist for a race from its
 * duration, elevation gain, maximum altitude and expected weather.
 *
 * The rules are applied in a fixed order, so the returned lists stay stable and
 * readable. Items are deduplicated by id: when the same id would appear in both
 * lists, the mandatory entry wins and is kept only once.
 *
 * Duration, D+ and maximum altitude that are negative or non-finite are treated
 * as zero, so malformed input degrades to "no extra gear" instead of throwing.
 */
export function calculateRequiredGear(context: GearContext): GearChecklist {
  const durationHours = nonNegative(context.durationHours);
  const dPlusM = nonNegative(context.dPlusM);
  const maxAltitudeM = nonNegative(context.maxAltitudeM);

  const candidates: GearCandidate[] = [];

  if (durationHours > 3) {
    candidates.push(
      makeItem('whistle', 'Silbato', 'mandatory', 'Salidas de más de 3 horas'),
      makeItem('thermal-blanket', 'Manta térmica', 'mandatory', 'Salidas de más de 3 horas'),
    );
  }

  if (durationHours > 6) {
    candidates.push(
      makeItem('first-aid', 'Botiquín de emergencia', 'mandatory', 'Salidas de más de 6 horas'),
      makeItem(
        'water-reserve',
        'Reserva de agua (mín. 1,5 L)',
        'mandatory',
        'Salidas de más de 6 horas',
      ),
    );
  } else if (durationHours > 2) {
    candidates.push(
      makeItem('soft-flasks', 'Bidones de 500 ml (x2)', 'recommended', 'Salidas de más de 2 horas'),
    );
  }

  if (dPlusM > 800 || (dPlusM > 500 && durationHours > 4)) {
    candidates.push(
      makeItem('poles', 'Bastones de trail', 'recommended', 'Desnivel acumulado alto'),
    );
  }

  if (maxAltitudeM > 2000) {
    candidates.push(
      makeItem('windbreaker', 'Chaqueta cortavientos', 'mandatory', 'Por encima de 2000 m'),
    );
  }

  const coldOrRainy =
    (context.minTemperatureC !== undefined && context.minTemperatureC < 5) ||
    context.rainy === true;
  if (coldOrRainy) {
    candidates.push(
      makeItem('waterproof', 'Chaqueta impermeable', 'mandatory', 'Frío o lluvia previstos'),
      makeItem('gloves', 'Guantes térmicos', 'recommended', 'Frío o lluvia previstos'),
    );
  }

  if (context.maxWindKmh !== undefined && context.maxWindKmh > 30) {
    candidates.push(makeItem('buff', 'Tubular / Buff', 'recommended', 'Viento fuerte'));
  }

  return dedupe(candidates);
}

/**
 * Keeps the first candidate per id, preserving rule order, and drops any
 * recommended candidate whose id is also produced as mandatory.
 */
function dedupe(candidates: GearCandidate[]): GearChecklist {
  const mandatoryIds = new Set<string>();
  for (const candidate of candidates) {
    if (candidate.category === 'mandatory') mandatoryIds.add(candidate.item.id);
  }

  const checklist: GearChecklist = { mandatory: [], recommended: [] };
  const seen = new Set<string>();

  for (const candidate of candidates) {
    const { id } = candidate.item;
    if (seen.has(id)) continue;
    if (candidate.category === 'recommended' && mandatoryIds.has(id)) continue;
    seen.add(id);
    checklist[candidate.category].push(candidate.item);
  }

  return checklist;
}
