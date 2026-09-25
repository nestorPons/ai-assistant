<?php

namespace App\Models\Kos;

use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * Registro auditable del mensaje original autorizado (tabla `raw_messages`).
 * La tabla no tiene `updated_at`; core-engine solo inserta y marca `processed`.
 */
class RawMessage extends Model
{
    use HasFactory, HasUuids;

    protected $table = 'raw_messages';

    protected $keyType = 'string';

    public $incrementing = false;

    public $timestamps = false;

    /** @var list<string> */
    protected $fillable = [
        'client_id',
        'source',
        'external_id',
        'content',
        'processed',
    ];

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'processed' => 'boolean',
            'created_at' => 'datetime',
        ];
    }

    public function client(): BelongsTo
    {
        return $this->belongsTo(Client::class, 'client_id');
    }
}
