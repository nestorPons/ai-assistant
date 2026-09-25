<?php

namespace App\Providers;

use App\Models\Kos\Client;
use App\Models\Kos\Task;
use App\Observers\ClientObserver;
use App\Observers\TaskObserver;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    /**
     * Register any application services.
     */
    public function register(): void
    {
        //
    }

    /**
     * Bootstrap any application services.
     */
    public function boot(): void
    {
        Client::observe(ClientObserver::class);
        Task::observe(TaskObserver::class);
    }
}
