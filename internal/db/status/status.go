// Package status define los tipos y constantes de estado para las tablas
// users e invites, reflejando exactamente los valores permitidos por los
// CHECK de la base de datos.
package status

// InviteStatus representa el estado de un usuario en users.invite_status.
// Valores válidos: pending, active, blocked (CHECK en 00001_users_invites.sql).
type InviteStatus string

const (
	// InviteStatusPending indica que el usuario aún no ha usado su invitación.
	InviteStatusPending InviteStatus = "pending"
	// InviteStatusActive indica que la invitación fue aceptada y el usuario tiene acceso.
	InviteStatusActive InviteStatus = "active"
	// InviteStatusBlocked indica que el usuario ha sido bloqueado.
	InviteStatusBlocked InviteStatus = "blocked"
)

// Status representa el estado de una invitación en invites.status.
// Valores válidos: pending, accepted, revoked, expired (CHECK en 00001_users_invites.sql).
type Status string

const (
	// StatusPending indica que la invitación está pendiente de uso.
	StatusPending Status = "pending"
	// StatusAccepted indica que la invitación fue aceptada.
	StatusAccepted Status = "accepted"
	// StatusRevoked indica que la invitación fue revocada manualmente.
	StatusRevoked Status = "revoked"
	// StatusExpired indica que la invitación caducó por tiempo.
	StatusExpired Status = "expired"
)
