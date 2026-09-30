<?php

namespace App\Providers\Filament;

use App\Filament\Widgets\TskHubStatsOverview;
use App\Filament\Widgets\WhatsAppConnectionWidget;
use Filament\Http\Middleware\Authenticate;
use Filament\Http\Middleware\AuthenticateSession;
use Filament\Http\Middleware\DisableBladeIconComponents;
use Filament\Http\Middleware\DispatchServingFilamentEvent;
use Filament\Pages\Dashboard;
use Filament\Panel;
use Filament\PanelProvider;
use Filament\Support\Colors\Color;
use Filament\View\PanelsRenderHook;
use Filament\Widgets\AccountWidget;
use Illuminate\Contracts\View\View;
use Illuminate\Cookie\Middleware\AddQueuedCookiesToResponse;
use Illuminate\Cookie\Middleware\EncryptCookies;
use Illuminate\Foundation\Http\Middleware\PreventRequestForgery;
use Illuminate\Routing\Middleware\SubstituteBindings;
use Illuminate\Session\Middleware\StartSession;
use Illuminate\View\Middleware\ShareErrorsFromSession;

class AdminPanelProvider extends PanelProvider
{
    public function panel(Panel $panel): Panel
    {
        return $panel
            ->default()
            ->id('admin')
            ->path('admin')
            ->login()
            ->spa()
            ->globalSearch(false)
            ->brandName('Tsk-Hub')
            ->colors([
                'primary' => Color::Emerald,
            ])
            ->discoverResources(in: app_path('Filament/Resources'), for: 'App\Filament\Resources')
            ->discoverPages(in: app_path('Filament/Pages'), for: 'App\Filament\Pages')
            ->pages([
                Dashboard::class,
            ])
            ->discoverWidgets(in: app_path('Filament/Widgets'), for: 'App\Filament\Widgets')
            ->widgets([
                WhatsAppConnectionWidget::class,
                TskHubStatsOverview::class,
                AccountWidget::class,
            ])
            ->renderHook(
                PanelsRenderHook::SIDEBAR_FOOTER,
                fn (): View => view('filament.sidebar-version'),
            )
            ->renderHook(
                PanelsRenderHook::TOPBAR_END,
                fn (): View => view('filament.topbar-version'),
            )
            ->renderHook(
                // Compacta filtros sobre el contenido y el buscador de las tablas.
                PanelsRenderHook::HEAD_END,
                fn (): string => <<<'HTML'
                    <style>
                        .fi-ta-filters-above-content-ctn { padding-block: 0.5rem; }
                        .fi-ta-filters-above-content-ctn .fi-ta-filters-header { display: none; }
                        .fi-ta-search-field { max-width: 12rem; }
                        /* Filtros + buscador de Tareas en una sola fila. */
                        .fi-ta-header-ctn:has(.fi-ta-filters-above-content-ctn) {
                            display: flex;
                            flex-wrap: wrap;
                            align-items: end;
                            column-gap: 1rem;
                            border-bottom: 1px solid #e5e7eb;
                        }
                        .dark .fi-ta-header-ctn:has(.fi-ta-filters-above-content-ctn) {
                            border-bottom-color: rgb(255 255 255 / 0.1);
                        }
                        .fi-ta-header-ctn:has(.fi-ta-filters-above-content-ctn) .fi-ta-filters-above-content-ctn {
                            border-bottom: 0;
                        }
                        .fi-ta-header-ctn:has(.fi-ta-filters-above-content-ctn) .fi-ta-header-toolbar {
                            
                            border-bottom: 0;
                        }
                        .task-status-filter input:not(:checked) + label.fi-btn.fi-color {
                            background-color: var(--bg);
                            color: var(--text);
                        }
                        .task-status-filter input:not(:checked)[value="needs_review"] + label {
                            background-color: var(--gray-100);
                            color: var(--gray-700);
                        }
                        .dark .task-status-filter input:not(:checked) + label.fi-btn.fi-color {
                            background-color: var(--dark-bg);
                            color: var(--dark-text);
                        }
                        .dark .task-status-filter input:not(:checked)[value="needs_review"] + label {
                            background-color: var(--gray-800);
                            color: var(--gray-200);
                        }
                    </style>
                    HTML,
            )
            ->middleware([
                EncryptCookies::class,
                AddQueuedCookiesToResponse::class,
                StartSession::class,
                AuthenticateSession::class,
                ShareErrorsFromSession::class,
                PreventRequestForgery::class,
                SubstituteBindings::class,
                DisableBladeIconComponents::class,
                DispatchServingFilamentEvent::class,
            ])
            ->authMiddleware([
                Authenticate::class,
            ]);
    }
}
