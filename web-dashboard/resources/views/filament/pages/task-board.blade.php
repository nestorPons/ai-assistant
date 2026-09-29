<x-filament-panels::page>
    @php($columns = $this->getColumns())
    @php($board = $this->getBoardData())
    @php($statuses = array_keys($columns))

    <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end">
        <div class="flex-1">
            <label class="fi-fo-field-label mb-1 block text-sm font-medium">Buscar</label>
            <input
                type="search"
                wire:model.live.debounce.300ms="search"
                placeholder="Título, tema, cliente…"
                class="fi-input block w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm dark:border-white/10 dark:bg-white/5"
            />
        </div>
        <div class="w-full sm:w-52">
            <label class="fi-fo-field-label mb-1 block text-sm font-medium">Prioridad</label>
            <select
                wire:model.live="priority"
                class="fi-input block w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm dark:border-white/10 dark:bg-white/5"
            >
                <option value="">Todas</option>
                <option value="high">Alta</option>
                <option value="medium">Media</option>
                <option value="low">Baja</option>
            </select>
        </div>
    </div>

    <div
        class="grid grid-cols-1 gap-4 md:grid-cols-3 xl:grid-cols-5"
        wire:poll.10s
    >
        @foreach ($columns as $status => $meta)
            <section
                class="flex min-h-64 flex-col rounded-xl border border-gray-200 bg-gray-50 dark:border-white/10 dark:bg-white/5"
                ondragover="event.preventDefault(); this.classList.add('ring-2', 'ring-primary-500')"
                ondragleave="this.classList.remove('ring-2', 'ring-primary-500')"
                ondrop="event.preventDefault(); this.classList.remove('ring-2', 'ring-primary-500'); const id = event.dataTransfer.getData('text/task-id'); if (id) { $wire.call('moveTask', id, '{{ $status }}'); }"
            >
                <header class="flex items-center justify-between gap-2 border-b border-gray-200 px-3 py-2.5 dark:border-white/10">
                    <h3 class="text-sm font-semibold">{{ $meta['label'] }}</h3>
                    <span class="fi-badge rounded-md bg-gray-200 px-2 py-0.5 text-xs font-medium dark:bg-white/10">
                        {{ $board[$status]->count() }}
                    </span>
                </header>

                <div class="flex flex-1 flex-col gap-2.5 p-3">
                    @forelse ($board[$status] as $task)
                        <article
                            draggable="true"
                            ondragstart="event.dataTransfer.setData('text/task-id', '{{ $task->getKey() }}'); event.dataTransfer.effectAllowed = 'move';"
                            class="cursor-grab rounded-lg border border-gray-200 bg-white p-3 shadow-sm transition hover:shadow active:cursor-grabbing dark:border-white/10 dark:bg-gray-900"
                        >
                            <div class="mb-1 flex items-center gap-2">
                                @php($priorityColor = $task->priority === 'high' ? 'bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400' : ($task->priority === 'medium' ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/10 dark:text-amber-400' : 'bg-gray-100 text-gray-600 dark:bg-white/10 dark:text-gray-300'))
                                <span class="rounded px-1.5 py-0.5 text-[11px] font-semibold uppercase {{ $priorityColor }}">
                                    {{ $task->priority }}
                                </span>
                                @if ($task->due_date)
                                    <span class="text-[11px] text-gray-500 dark:text-gray-400">
                                        {{ $task->due_date->format('Y-m-d') }}
                                    </span>
                                @endif
                                @if (! is_null($task->ai_confidence))
                                    <span class="ml-auto text-[11px] text-gray-400">
                                        {{ number_format(((float) $task->ai_confidence) * 100, 0) }}%
                                    </span>
                                @endif
                            </div>

                            <a
                                href="{{ $this->editUrl($task) }}"
                                wire:navigate
                                class="block text-sm font-semibold leading-snug hover:underline"
                            >
                                {{ $task->title }}
                            </a>

                            <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
                                {{ $task->subject }}
                                @if ($task->client)
                                    · {{ $task->client->name }}
                                @endif
                            </p>

                            <div class="mt-2 flex items-center gap-1">
                                @php($index = array_search($status, $statuses, true))
                                @if ($index > 0)
                                    <button
                                        type="button"
                                        wire:click="moveTask('{{ $task->getKey() }}', '{{ $statuses[$index - 1] }}')"
                                        title="Mover a {{ $columns[$statuses[$index - 1]]['label'] }}"
                                        class="rounded-md px-1.5 py-1 text-sm text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10"
                                    >
                                        ←
                                    </button>
                                @endif
                                <a
                                    href="{{ $this->editUrl($task) }}"
                                    wire:navigate
                                    class="ml-auto rounded-md px-2 py-1 text-xs font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:hover:bg-white/10"
                                >
                                    Abrir
                                </a>
                                @if ($index < count($statuses) - 1)
                                    <button
                                        type="button"
                                        wire:click="moveTask('{{ $task->getKey() }}', '{{ $statuses[$index + 1] }}')"
                                        title="Mover a {{ $columns[$statuses[$index + 1]]['label'] }}"
                                        class="rounded-md px-1.5 py-1 text-sm text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10"
                                    >
                                        →
                                    </button>
                                @endif
                            </div>
                        </article>
                    @empty
                        <p class="rounded-lg border border-dashed border-gray-300 p-4 text-center text-xs text-gray-400 dark:border-white/10">
                            Sin tareas
                        </p>
                    @endforelse
                </div>
            </section>
        @endforeach
    </div>

    <p class="mt-3 text-xs text-gray-400">
        Arrastra tarjetas entre columnas o usa ← → para cambiar el estado. Máx. 50 por columna. El cambio queda auditado.
    </p>
</x-filament-panels::page>
