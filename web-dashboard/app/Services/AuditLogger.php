<?php

namespace App\Services;

use App\Models\AdminAuditLog;
use Illuminate\Support\Facades\Auth;

/**
 * Registra acciones del panel en `admin_audit_logs`. No lanza si algo falla:
 * la auditoría nunca debe interrumpir la operación principal.
 */
class AuditLogger
{
    /**
     * @param  array<string, mixed>  $changes
     */
    public function log(
        string $action,
        ?string $subjectType = null,
        int|string|null $subjectId = null,
        array $changes = [],
        ?string $user = null,
    ): void {
        try {
            $request = request();

            AdminAuditLog::create([
                'user' => $user ?? Auth::user()?->email ?? 'system',
                'action' => $action,
                'subject_type' => $subjectType,
                'subject_id' => $subjectId !== null ? (string) $subjectId : null,
                'changes' => $changes === [] ? null : $changes,
                'ip' => $request?->ip(),
                'user_agent' => $request?->userAgent() !== null
                    ? mb_substr((string) $request->userAgent(), 0, 255)
                    : null,
            ]);
        } catch (\Throwable) {
            // Se ignora cualquier fallo de auditoría para no afectar al flujo.
        }
    }
}
