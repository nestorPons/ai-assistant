<?php

namespace Database\Factories\Kos;

use App\Models\Kos\Client;
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
