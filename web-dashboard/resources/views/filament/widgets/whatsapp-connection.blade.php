@php
    use Filament\Support\View\ComponentAttributeBag;

    $pollingInterval = $this->getPollingInterval();
    $connected = (bool) ($status['connected'] ?? false);
    $error = $status['error'] ?? null;
    $qrDataUri = $qrDataUri ?? null;
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
        <x-slot name="heading">WhatsApp</x-slot>
        <x-slot name="description">Conexión del canal de ingesta de WhatsApp</x-slot>

        @if ($connected)
            <div class="flex items-center gap-2">
                <x-filament::badge color="success">Conectado</x-filament::badge>
                <span class="text-sm text-gray-500 dark:text-gray-400">Recibiendo mensajes de los contactos autorizados.</span>
            </div>
        @else
            <div class="flex flex-col items-center gap-4 py-2">
                @if ($qrDataUri)
                    <img
                        src="{{ $qrDataUri }}"
                        alt="Código QR de WhatsApp"
                        class="h-64 w-64 rounded-lg border border-gray-200 dark:border-gray-700"
                    >
                    <p class="text-center text-sm text-gray-500 dark:text-gray-400">
                        Escanea el código en WhatsApp: <strong>Ajustes → Dispositivos vinculados</strong>.
                    </p>
                @else
                    <x-filament::badge color="danger">Desconectado</x-filament::badge>
                    @if ($error)
                        <p class="text-center text-sm text-gray-500 dark:text-gray-400">{{ $error }}</p>
                    @else
                        <p class="text-center text-sm text-gray-500 dark:text-gray-400">
                            Esperando el código QR del motor, reintentando…
                        </p>
                    @endif
                @endif
            </div>
        @endif
    </x-filament::section>
</x-filament-widgets::widget>
