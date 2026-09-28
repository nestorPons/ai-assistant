<?php

namespace Database\Factories\TskHub;

use App\Models\TskHub\Client;
use Illuminate\Database\Eloquent\Factories\Factory;

/**
 * @extends Factory<Client>
 */
class ClientFactory extends Factory
{
    protected $model = Client::class;

    /**
     * @return array<string, mixed>
     */
    public function definition(): array
    {
        return [
            'source' => 'gmail',
            'identifier' => fake()->unique()->safeEmail(),
            'name' => fake()->name(),
            'tracked' => true,
            'active' => true,
        ];
    }
}
