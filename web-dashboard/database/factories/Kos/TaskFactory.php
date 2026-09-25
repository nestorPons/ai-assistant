<?php

namespace Database\Factories\Kos;

use App\Models\Kos\Task;
use Illuminate\Database\Eloquent\Factories\Factory;

/**
 * @extends Factory<Task>
 */
class TaskFactory extends Factory
{
    protected $model = Task::class;

    /**
     * @return array<string, mixed>
     */
    public function definition(): array
    {
        return [
            'client_id' => ClientFactory::new(),
            'message_id' => RawMessageFactory::new(),
            'title' => fake()->sentence(4),
            'description' => fake()->paragraph(),
            'subject' => fake()->domainName(),
            'priority' => 'medium',
            'estimated_hours' => null,
            'due_date' => null,
            'specifications' => null,
            'status' => 'pending',
            'ai_confidence' => 0.9,
        ];
    }
}
