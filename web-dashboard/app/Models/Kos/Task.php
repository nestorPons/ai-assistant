<?php

namespace App\Models\Kos;

use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * Tarea procesada por core-engine (tabla `tasks`).
 */
class Task extends Model
{
    use HasFactory, HasUuids;

    public const PRIORITIES = ['low', 'medium', 'high'];

    public const STATUSES = ['pending', 'in_progress', 'completed', 'discarded', 'needs_review'];

    protected $table = 'tasks';

    protected $keyType = 'string';

    public $incrementing = false;

    /** @var list<string> */
    protected $fillable = [
        'client_id',
        'message_id',
        'title',
        'description',
        'subject',
        'priority',
        'estimated_hours',
        'due_date',
        'specifications',
        'status',
        'ai_confidence',
    ];

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'estimated_hours' => 'float',
            'due_date' => 'date',
            'specifications' => 'array',
            'ai_confidence' => 'float',
            'created_at' => 'datetime',
            'updated_at' => 'datetime',
        ];
    }

    public function client(): BelongsTo
    {
        return $this->belongsTo(Client::class, 'client_id');
    }

    public function message(): BelongsTo
    {
        return $this->belongsTo(RawMessage::class, 'message_id');
    }
}
