@php
    use Filament\Support\View\ComponentAttributeBag;

    $pollingInterval = $this->getPollingInterval();
    $labels = ['gmail' => 'Gmail', 'telegram' => 'Telegram', 'whatsapp' => 'WhatsApp'];
@endphp

<x-filament-widgets::widget
    :attributes="
        (new ComponentAttributeBag)
            ->merge([
                'wire:poll.' . $pollingInterval => $pollingInterval ? true : null,
            ], escape: false)
    "
>
    <x-filament::section>
        <x-slot name="heading">Canales de comunicación</x-slot>
        <x-slot name="description">Estado de la conexión al iniciar la aplicación</x-slot>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
            @foreach ($channels as $channel)
                @php
                    $name = $channel['name'] ?? '';
                    $label = $labels[$name] ?? ucfirst((string) $name);
                    $enabled = (bool) ($channel['enabled'] ?? false);
                    $connected = (bool) ($channel['connected'] ?? false);
                    $mode = $channel['mode'] ?? '';
                    $detail = $channel['detail'] ?? '';
                    $error = $channel['error'] ?? '';
                @endphp
                <div class="flex flex-col gap-2 rounded-lg border border-gray-200 p-4 dark:border-gray-700">
                    <div class="flex items-center justify-between gap-2">
                        <span class="text-sm font-semibold">{{ $label }}</span>
                        @if ($connected)
                            <x-filament::badge color="success">Conectado</x-filament::badge>
                        @elseif ($enabled)
                            <x-filament::badge color="danger">Desconectado</x-filament::badge>
                        @else
                            <x-filament::badge color="gray">Deshabilitado</x-filament::badge>
                        @endif
                    </div>
                    @if ($mode !== '' || $detail !== '')
                        <span class="text-sm text-gray-500 dark:text-gray-400">
                            {{ $mode }}@if ($mode !== '' && $detail !== '') · @endif{{ $detail }}
                        </span>
                    @endif
                    @if (! $connected && $error !== '')
                        <span class="text-sm text-gray-500 dark:text-gray-400">{{ $error }}</span>
                    @endif
                </div>
            @endforeach
        </div>
    </x-filament::section>
</x-filament-widgets::widget>
