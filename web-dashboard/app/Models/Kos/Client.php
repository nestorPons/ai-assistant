<?php

namespace App\Models\Kos;

use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

/**
 * Contacto de la lista blanca gestionado por core-engine (tabla `clients`).
 */
class Client extends Model
{
    use HasFactory, HasUuids;

    protected $table = 'clients';

    protected $keyType = 'string';

    public $incrementing = false;

    /** @var list<string> */
    protected $fillable = [
        'source',
        'identifier',
        'name',
        'tracked',
        'active',
    ];

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'tracked' => 'boolean',
            'active' => 'boolean',
            'created_at' => 'datetime',
            'updated_at' => 'datetime',
        ];
    }

    public function rawMessages(): HasMany
    {
        return $this->hasMany(RawMessage::class, 'client_id');
    }

    public function tasks(): HasMany
    {
        return $this->hasMany(Task::class, 'client_id');
    }

    public function isAuthorized(): bool
    {
        return $this->tracked && $this->active;
    }
}
