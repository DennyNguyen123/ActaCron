#ifndef MyAppVersion
  #define MyAppVersion "v1.0.1"
#endif

[Setup]
AppId={{D8C8E192-3B47-49F1-8B0C-1A6E1E4F22A0}
AppName=ActaCron
AppVersion={#MyAppVersion}
AppPublisher=DennyNguyen
AppPublisherURL=https://github.com/DennyNguyen123/ActaCron
AppSupportURL=https://github.com/DennyNguyen123/ActaCron/issues
DefaultDirName={localappdata}\Programs\ActaCron
DefaultGroupName=ActaCron
OutputBaseFilename=ActaCron-Setup-{#MyAppVersion}
OutputDir=..\dist
Compression=lzma2/ultra64
SolidCompression=yes
PrivilegesRequired=lowest
CloseApplications=yes
RestartApplications=no
WizardStyle=modern
UninstallDisplayIcon={app}\actacron.exe

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "autostart"; Description: "Start ActaCron automatically when Windows starts"; GroupDescription: "Startup:"; Flags: unchecked

[Dirs]
Name: "{app}\packages"; Flags: uninsneveruninstall

[Files]
Source: "..\dist\actacron-{#MyAppVersion}-windows-amd64\actacron.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\actacron-{#MyAppVersion}-windows-amd64\.env.example"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\actacron-{#MyAppVersion}-windows-amd64\README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme
Source: "..\dist\actacron-{#MyAppVersion}-windows-amd64\packages\*"; DestDir: "{app}\packages"; Flags: skipifsourcedoesntexist onlyifdoesntexist recursesubdirs createallsubdirs

[Icons]
Name: "{group}\ActaCron"; Filename: "{app}\actacron.exe"
Name: "{group}\{cm:UninstallProgram,ActaCron}"; Filename: "{uninstallexe}"
Name: "{userdesktop}\ActaCron"; Filename: "{app}\actacron.exe"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "ActaCron"; ValueData: """{app}\actacron.exe"""; Flags: uninsdeletevalue; Tasks: autostart

[Run]
Filename: "{app}\actacron.exe"; Description: "{cm:LaunchProgram,ActaCron}"; Flags: nowait postinstall skipifsilent
