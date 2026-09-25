<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Model;

/**
 * Registro de auditoría de las acciones realizadas en el panel web.
 */
class AdminAuditLog extends Model
{
    use HasUuids;

    protected $table = 'admin_audit_logs';

    protected $keyType = 'string';

    public $incrementing = false;

    public const UPDATED_AT = null;

    /** @var list<string> */
    protected $fillable = [
        'user',
        'action',
        'subject_type',
        'subject_id',
        'changes',
        'ip',
        'user_agent',
    ];

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'changes' => 'array',
            'created_at' => 'datetime',
        ];
    }
}
