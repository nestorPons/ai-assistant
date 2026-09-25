<?php

namespace App\Filament\Resources\AdminAuditLogs;

use App\Filament\Resources\AdminAuditLogs\Pages\ListAdminAuditLogs;
use App\Filament\Resources\AdminAuditLogs\Pages\ViewAdminAuditLog;
use App\Filament\Resources\AdminAuditLogs\Schemas\AdminAuditLogInfolist;
use App\Filament\Resources\AdminAuditLogs\Tables\AdminAuditLogsTable;
use App\Models\AdminAuditLog;
use BackedEnum;
use Filament\Resources\Resource;
use Filament\Schemas\Schema;
use Filament\Support\Icons\Heroicon;
use Filament\Tables\Table;

/**
 * Auditoría del panel. Solo lectura.
 */
class AdminAuditLogResource extends Resource
{
    protected static ?string $model = AdminAuditLog::class;

    protected static string|BackedEnum|null $navigationIcon = Heroicon::OutlinedClipboardDocumentCheck;

    protected static string|\UnitEnum|null $navigationGroup = 'K-OS';

    protected static ?string $navigationLabel = 'Logs';

    protected static ?string $modelLabel = 'Registro';

    protected static ?string $pluralModelLabel = 'Logs';

    protected static ?int $navigationSort = 40;

    public static function infolist(Schema $schema): Schema
    {
        return AdminAuditLogInfolist::configure($schema);
    }

    public static function table(Table $table): Table
    {
        return AdminAuditLogsTable::configure($table);
    }

    public static function getRelations(): array
    {
        return [
            //
        ];
    }

    public static function getPages(): array
    {
        return [
            'index' => ListAdminAuditLogs::route('/'),
            'view' => ViewAdminAuditLog::route('/{record}'),
        ];
    }
}
