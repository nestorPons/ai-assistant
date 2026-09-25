<?php

namespace Database\Factories\Kos;

use App\Models\Kos\RawMessage;
use Illuminate\Database\Eloquent\Factories\Factory;
use Illuminate\Support\Str;

/**
 * @extends Factory<RawMessage>
 */
class RawMessageFactory extends Factory
{
    protected $model = RawMessage::class;

    /**
     * @return array<string, mixed>
     */
    public function definition(): array
    {
        return [
            'client_id' => ClientFactory::new(),
            'source' => 'gmail',
            'external_id' => (string) Str::uuid(),
            'content' => fake()->paragraph(),
            'processed' => false,
            'created_at' => now(),
        ];
    }
}
