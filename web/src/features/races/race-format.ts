export const format = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 1 });
export const km = (metres: number) =>
  `${new Intl.NumberFormat('es-ES', { maximumFractionDigits: 3 }).format(metres / 1000)} km`;
export const metres = (gain: number) => `${format.format(gain)} m`;

export function countdown(date: string, now = new Date()) {
  const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
  const days = (Date.parse(date) - Date.parse(today)) / 86_400_000;
  return days < 0
    ? 'Pasada'
    : days === 0
      ? 'Hoy'
      : days === 1
        ? 'Falta 1 día'
        : `Faltan ${days} días`;
}
