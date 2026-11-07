import { describe, expect, it } from 'vitest';
import { calculateRequiredGear } from './gear';
import type { GearChecklist, GearContext, GearItem } from './types';

function context(overrides: Partial<GearContext> = {}): GearContext {
  return {
    durationHours: 1,
    dPlusM: 0,
    maxAltitudeM: 0,
    ...overrides,
  };
}

function ids(items: GearItem[]): string[] {
  return items.map((item) => item.id);
}

function byId(checklist: GearChecklist, id: string): GearItem | undefined {
  return [...checklist.mandatory, ...checklist.recommended].find((item) => item.id === id);
}

describe('calculateRequiredGear', () => {
  it('returns no gear for a short, easy outing', () => {
    const checklist = calculateRequiredGear(
      context({ durationHours: 1, dPlusM: 100, maxAltitudeM: 500 }),
    );

    expect(checklist.mandatory).toEqual([]);
    expect(checklist.recommended).toEqual([]);
  });

  it('requires a whistle and a thermal blanket beyond three hours', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 3.5 }));

    expect(checklist.mandatory).toEqual([
      {
        id: 'whistle',
        name: 'Silbato',
        category: 'mandatory',
        reason: 'Salidas de más de 3 horas',
      },
      {
        id: 'thermal-blanket',
        name: 'Manta térmica',
        category: 'mandatory',
        reason: 'Salidas de más de 3 horas',
      },
    ]);
    // The 3.5 h outing also falls in the 2–6 h window, so soft flasks apply.
    expect(ids(checklist.recommended)).toEqual(['soft-flasks']);
  });

  it('requires first aid and water reserve beyond six hours', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 7 }));

    expect(ids(checklist.mandatory)).toEqual([
      'whistle',
      'thermal-blanket',
      'first-aid',
      'water-reserve',
    ]);
    expect(byId(checklist, 'first-aid')).toMatchObject({
      name: 'Botiquín de emergencia',
      category: 'mandatory',
      reason: 'Salidas de más de 6 horas',
    });
    expect(byId(checklist, 'water-reserve')).toMatchObject({
      name: 'Reserva de agua (mín. 1,5 L)',
      category: 'mandatory',
      reason: 'Salidas de más de 6 horas',
    });
    // Soft flasks are the short-outing alternative and must not appear at 7 h.
    expect(checklist.recommended).toEqual([]);
  });

  it('recommends soft flasks for a 2.5 hour outing', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 2.5 }));

    expect(checklist.mandatory).toEqual([]);
    expect(checklist.recommended).toEqual([
      {
        id: 'soft-flasks',
        name: 'Bidones de 500 ml (x2)',
        category: 'recommended',
        reason: 'Salidas de más de 2 horas',
      },
    ]);
  });

  it('recommends poles for high elevation gain', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 2, dPlusM: 900 }));

    expect(ids(checklist.recommended)).toContain('poles');
    expect(byId(checklist, 'poles')).toMatchObject({
      name: 'Bastones de trail',
      reason: 'Desnivel acumulado alto',
    });
  });

  it('recommends poles for moderate gain on a long outing', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 5, dPlusM: 600 }));

    expect(ids(checklist.recommended)).toContain('poles');
  });

  it('does not recommend poles for moderate gain on a short outing', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 3, dPlusM: 600 }));

    expect(ids(checklist.recommended)).not.toContain('poles');
  });

  it('requires a windbreaker above 2000 m', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 1, maxAltitudeM: 2500 }));

    expect(ids(checklist.mandatory)).toContain('windbreaker');
    expect(byId(checklist, 'windbreaker')).toMatchObject({
      name: 'Chaqueta cortavientos',
      category: 'mandatory',
      reason: 'Por encima de 2000 m',
    });
  });

  it('does not require a windbreaker at or below 2000 m', () => {
    expect(
      ids(calculateRequiredGear(context({ durationHours: 1, maxAltitudeM: 1500 })).mandatory),
    ).not.toContain('windbreaker');
  });

  it('adds waterproof and gloves exactly once when it is cold', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 1, minTemperatureC: 3 }));

    expect(ids(checklist.mandatory)).toEqual(['waterproof']);
    expect(ids(checklist.recommended)).toEqual(['gloves']);
    expect(byId(checklist, 'waterproof')).toMatchObject({
      name: 'Chaqueta impermeable',
      reason: 'Frío o lluvia previstos',
    });
    expect(byId(checklist, 'gloves')).toMatchObject({
      name: 'Guantes térmicos',
      reason: 'Frío o lluvia previstos',
    });
  });

  it('adds waterproof and gloves exactly once when it rains', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 1, rainy: true }));

    expect(ids(checklist.mandatory)).toEqual(['waterproof']);
    expect(ids(checklist.recommended)).toEqual(['gloves']);
  });

  it('does not duplicate waterproof or gloves when it is cold and rainy', () => {
    const checklist = calculateRequiredGear(
      context({ durationHours: 1, minTemperatureC: 3, rainy: true }),
    );

    expect(ids(checklist.mandatory).filter((id) => id === 'waterproof')).toHaveLength(1);
    expect(ids(checklist.recommended).filter((id) => id === 'gloves')).toHaveLength(1);
  });

  it('does not add waterproof when it is mild and dry', () => {
    const checklist = calculateRequiredGear(
      context({ durationHours: 1, minTemperatureC: 12, rainy: false }),
    );

    expect(ids(checklist.mandatory)).not.toContain('waterproof');
    expect(ids(checklist.recommended)).not.toContain('gloves');
  });

  it('recommends a buff in strong wind', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 1, maxWindKmh: 40 }));

    expect(ids(checklist.recommended)).toEqual(['buff']);
    expect(byId(checklist, 'buff')).toMatchObject({
      name: 'Tubular / Buff',
      reason: 'Viento fuerte',
    });
  });

  it('does not recommend a buff in light wind', () => {
    const checklist = calculateRequiredGear(context({ durationHours: 1, maxWindKmh: 20 }));

    expect(ids(checklist.recommended)).not.toContain('buff');
  });

  it('never lists the same id in both mandatory and recommended for a maximal context', () => {
    const checklist = calculateRequiredGear(
      context({
        durationHours: 7,
        dPlusM: 900,
        maxAltitudeM: 2500,
        minTemperatureC: 3,
        maxWindKmh: 40,
        rainy: true,
      }),
    );

    const mandatoryIds = ids(checklist.mandatory);
    const recommendedIds = ids(checklist.recommended);

    expect(mandatoryIds).toEqual([
      'whistle',
      'thermal-blanket',
      'first-aid',
      'water-reserve',
      'windbreaker',
      'waterproof',
    ]);
    expect(recommendedIds).toEqual(['poles', 'gloves', 'buff']);

    // Deduplication guarantees no id is returned in both lists, and every id
    // is unique across the whole checklist.
    const allIds = [...mandatoryIds, ...recommendedIds];
    expect(mandatoryIds.filter((id) => recommendedIds.includes(id))).toEqual([]);
    expect(new Set(allIds).size).toBe(allIds.length);
  });

  it('handles exact rule thresholds without crossing them', () => {
    const exactlyThree = calculateRequiredGear(context({ durationHours: 3 }));
    expect(ids(exactlyThree.mandatory)).not.toContain('whistle');

    const exactlySix = calculateRequiredGear(context({ durationHours: 6 }));
    expect(ids(exactlySix.mandatory)).not.toContain('first-aid');
    expect(ids(exactlySix.recommended)).toContain('soft-flasks');

    const exactlyTwo = calculateRequiredGear(context({ durationHours: 2 }));
    expect(ids(exactlyTwo.recommended)).not.toContain('soft-flasks');

    const exactlyTwoThousand = calculateRequiredGear(context({ maxAltitudeM: 2000 }));
    expect(ids(exactlyTwoThousand.mandatory)).not.toContain('windbreaker');

    const exactlyFiveDegrees = calculateRequiredGear(context({ minTemperatureC: 5 }));
    expect(ids(exactlyFiveDegrees.mandatory)).not.toContain('waterproof');

    const exactlyThirtyWind = calculateRequiredGear(context({ maxWindKmh: 30 }));
    expect(ids(exactlyThirtyWind.recommended)).not.toContain('buff');

    const exactlyEightHundred = calculateRequiredGear(context({ durationHours: 2, dPlusM: 800 }));
    expect(ids(exactlyEightHundred.recommended)).not.toContain('poles');
  });

  it('treats non-finite and negative numeric inputs as zero without throwing', () => {
    const nonFinite = calculateRequiredGear({
      durationHours: Number.NaN,
      dPlusM: -100,
      maxAltitudeM: Number.POSITIVE_INFINITY,
    });
    expect(nonFinite).toEqual({ mandatory: [], recommended: [] });

    const negative = calculateRequiredGear({
      durationHours: -5,
      dPlusM: Number.NEGATIVE_INFINITY,
      maxAltitudeM: -2000,
    });
    expect(negative).toEqual({ mandatory: [], recommended: [] });
  });
});
