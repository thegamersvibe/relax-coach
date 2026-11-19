#!/usr/bin/env python3
"""Build the authorized combined guild archive. No network or publication."""
from pathlib import Path
import argparse,hashlib,json,re,shutil,zipfile
ROOT=Path(__file__).resolve().parents[1]
REACTION_AUDIO={'Interface/AddOns/ForeverCoach/Media/Reactions/'+name+'.mp3' for name in ('level_up','bags_1','bags_2','death','campfire')}
# 1.5.6 replaced the one campfire song with two that take turns.
REACTION_AUDIO_156=(REACTION_AUDIO-{'Interface/AddOns/ForeverCoach/Media/Reactions/campfire.mp3'})|{'Interface/AddOns/ForeverCoach/Media/Reactions/campfire_'+n+'.mp3' for n in ('1','2')}
REACTION_AUDIO_170=REACTION_AUDIO_156|{'Interface/AddOns/ForeverCoach/Media/Reactions/flight.mp3'}
def reaction_audio_for(version):
    if tuple(map(int,version.split('.'))) >= (1,7,0):return REACTION_AUDIO_170
    return REACTION_AUDIO_156 if tuple(map(int,version.split('.'))) >= (1,5,6) else REACTION_AUDIO

def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()

def raid_release(version):
    # The app owns the floor. Never guess it when the source is missing or changed.
    try:source=(ROOT/'desktop/addon_update.go').read_text(encoding='utf-8')
    except (OSError,UnicodeError):raise SystemExit('Cannot parse addonRaidMinimumVersion from desktop/addon_update.go')
    found=re.findall(r'^const addonRaidMinimumVersion = "((?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*))"$',source,re.M)
    if len(found)!=1:raise SystemExit('Cannot parse addonRaidMinimumVersion from desktop/addon_update.go')
    return tuple(map(int,version.split('.'))) >= tuple(map(int,found[0].split('.')))

def check_raid_path(name):
    # addonSafeZipName: validate load paths and shipped filenames identically.
    if not name or len(name)>200 or name.startswith('/') or any(c in name for c in '\\:\x00'):
        raise SystemExit('Unsafe Relax Raid path: '+name)
    for part in name.removesuffix('/').split('/'):
        base=part.split('.',1)[0].upper()
        if (not part or part in {'.','..'} or part.rstrip(' .')!=part
            or any(ord(c)<32 or ord(c)>126 or c in '<>"|?*' for c in part)
            or base in {'CON','PRN','AUX','NUL'} or re.fullmatch(r'(COM|LPT)[0-9]',base)):
            raise SystemExit('Unsafe Relax Raid path: '+name)

def check_raid_toc(names):
    # Mirror addonTOCMetadata / addonCheckRaidTOC; keep legacy roots unchanged.
    toc=ROOT/'addon/RelaxRaid/RelaxRaid.toc'
    try:
        if not toc.is_file() or toc.stat().st_size>64<<10:raise SystemExit('Relax Raid TOC is not a small regular file')
        with toc.open('rb') as source:data=source.read((64<<10)+1)
        if len(data)>64<<10:raise SystemExit('Relax Raid TOC is not a small regular file')
        text=data.decode('utf-8')
    except (OSError,UnicodeError):raise SystemExit('Invalid Relax Raid TOC text or missing RelaxRaid.toc')
    if '\x00' in text:raise SystemExit('Invalid Relax Raid TOC text')
    meta={};loads=[]
    for line in text.split('\n'):
        line=line.strip()
        if line.startswith('##'):
            key,sep,value=line[2:].strip().partition(':')
            if not sep:continue
            key=key.strip().lower()
            if key in meta:raise SystemExit('duplicate addon TOC metadata: '+key)
            meta[key]=value.strip()
        elif line and not line.startswith('#'):loads.append(line.replace('\\','/'))
    if meta.get('title')!='Relax Raid' or not meta.get('version') or meta.get('savedvariables')!='RelaxRaidDB' or meta.get('savedvariablespercharacter')!='RelaxRaidCharDB':
        raise SystemExit('incompatible Relax Raid TOC identity/version/SavedVariables')
    for key in ('dependencies','requireddeps','requireddependencies','deps','required-dependencies','required-deps'):
        if meta.get(key):raise SystemExit('incompatible Relax Raid TOC: it must not require other addons')
    for name in loads:
        check_raid_path(name)
        if 'Interface/AddOns/RelaxRaid/'+name not in names:raise SystemExit('missing Relax Raid TOC load file: '+name)
    if not loads:raise SystemExit('incomplete Relax Raid runtime')
    return meta['version']

def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('--exe',type=Path,required=True)
    ap.add_argument('--version',default='1.8.0')
    a=ap.parse_args()
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+',a.version):raise SystemExit('Invalid version')
    if not a.exe.is_file() or a.exe.read_bytes()[:2]!=b'MZ':raise SystemExit('Provide the verified Windows EXE')
    # From 1.5.0 the bundle also carries Relax Quest Planner under a new asset
    # name, so apps before 1.5.0 (which refuse that folder) ask for Update App.
    bundle=tuple(map(int,a.version.split('.')))>=(1,5,0)
    raid=raid_release(a.version)
    legacy=ROOT/'dist'/f'RelaxCoach-Bundle-{a.version}.zip'
    if raid and (legacy.exists() or legacy.is_symlink()):raise SystemExit('Refusing stale legacy bundle: '+legacy.name)
    addons=[('ForeverCoach','Relax Coach'),('Journey','Relax Journey')]+([('RelaxQuestPlanner','Relax Quest Planner')] if bundle else [])
    for folder,title in addons:
        if '## Title: '+title not in (ROOT/'addon'/folder/(folder+'.toc')).read_text():raise SystemExit('Unexpected addon identity')
    if raid:addons.append(('RelaxRaid','Relax Raid'))
    planner_version=None
    if bundle:
        found=re.search(r'^## Version: *(\S+?) *\r?$',(ROOT/'addon/RelaxQuestPlanner/RelaxQuestPlanner.toc').read_text(),re.M)
        if not found:raise SystemExit('Relax Quest Planner version missing')
        planner_version=found.group(1)
    if '## Version: '+a.version not in (ROOT/'addon/ForeverCoach/ForeverCoach.toc').read_text():raise SystemExit('Coach version mismatch')
    inputs=[]
    for folder,_ in addons:
        for p in sorted((ROOT/'addon'/folder).rglob('*')):
            if p.is_file():inputs.append((p,'Interface/AddOns/'+p.relative_to(ROOT/'addon').as_posix()))
    content=ROOT/'content/Journey-Content'
    if not content.is_dir() or not any(content.glob('*.html')):raise SystemExit('Journey guide content missing')
    for p in sorted(content.rglob('*')):
        if p.is_file():inputs.append((p,'Interface/Content/'+p.relative_to(ROOT/'content').as_posix()))
    inputs.extend([(a.exe,'RelaxCoach.exe'),(ROOT/'README.md','README.md'),(ROOT/'LICENSE','LICENSE-Coach')])
    inputs.extend((ROOT/'scripts'/name,name) for name in ('Install-RelaxCoach.ps1','Restore-RelaxCoach.ps1'))
    names=set()
    raid_spellings={}
    for p,name in inputs:
        parts=name.split('/')
        if p.is_symlink() or any(parent.is_symlink() for parent in p.parents):raise SystemExit('Symlink input rejected: '+name)
        if any(x.lower() in {'wtf','private','evidence','backups','.git','__pycache__'} for x in parts):raise SystemExit('Private path rejected: '+name)
        if name.startswith('/') or '..' in parts or '\\' in name or ':' in name or name in names:raise SystemExit('Unsafe or duplicate path: '+name)
        if name.startswith('Interface/Content/') and p.suffix!='.html':raise SystemExit('Unexpected guide file: '+name)
        for root,toc in (('Interface/AddOns/Journey/','Journey.toc'),('Interface/AddOns/RelaxQuestPlanner/','RelaxQuestPlanner.toc'),('Interface/AddOns/RelaxRaid/','RelaxRaid.toc')):
            if name.startswith(root) and name.lower().endswith('.toc') and name!=root+toc:raise SystemExit('Extra addon TOC rejected: '+name)
        if name.startswith('Interface/AddOns/RelaxRaid/'):
            check_raid_path(name)
            for end in range(1,len(parts)+1):
                component='/'.join(parts[:end]);key=component.lower()
                if key in raid_spellings and raid_spellings[key]!=component:raise SystemExit('Case-colliding Relax Raid path: '+component)
                raid_spellings[key]=component
            if p.suffix.lower() not in {'.lua','.toc','.md','.txt'}:raise SystemExit('Unexpected Relax Raid file: '+name)
        reaction_audio=name in reaction_audio_for(a.version) and tuple(map(int,a.version.split('.'))) >= (1,4,3)
        if name.startswith('Interface/AddOns/') and not reaction_audio and p.suffix.lower() not in {'.lua','.toc','.tga','.blp','.xml','.txt','.md','.ttf','.otf','.html','.png'}:raise SystemExit('Unexpected addon file: '+name)
        names.add(name)
    if tuple(map(int,a.version.split('.'))) >= (1,7,0):
        missing=reaction_audio_for(a.version)-names
        if missing:raise SystemExit('Missing reaction audio: '+', '.join(sorted(missing)))
    raid_version=check_raid_toc(names) if raid else None
    rows=[{'path':name,'sha256':digest(p),'bytes':p.stat().st_size} for p,name in inputs]
    manifest={'project':'Relax Coach','kind':'guild-combined','version':a.version,'journeyVersion':'4.00.04','contentVersion':'4.00.12'}
    if planner_version:manifest['plannerVersion']=planner_version
    if raid_version:manifest['raidVersion']=raid_version
    manifest['files']=rows
    dist=ROOT/'dist';dist.mkdir(exist_ok=True)
    standalone=dist/'RelaxCoach.exe'
    # Preserve the validated EXE before removing a previous package.
    if not standalone.exists() or not a.exe.samefile(standalone):shutil.copy2(a.exe,standalone)
    inputs=[(standalone if name=='RelaxCoach.exe' else p,name) for p,name in inputs]
    pkg=dist/'guild-package'
    if pkg.exists():shutil.rmtree(pkg)
    for p,name in inputs:
        target=pkg/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(p,target)
    (pkg/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    archive=dist/(f'RelaxCoach-RaidBundle-{a.version}.zip' if raid else f'RelaxCoach-Bundle-{a.version}.zip' if bundle else f'RelaxCoach-Guild-{a.version}.zip')
    with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
        for p in sorted(pkg.rglob('*')):
            if p.is_file():z.write(p,p.relative_to(pkg).as_posix())
    coach=dist/'public'/f'RelaxCoach-{a.version}.zip'
    assets=[archive,standalone]
    # Pre-1.4.2 apps understand only Coach-only archives. New releases must not
    # advertise a leftover one and let old apps report success without guides.
    if tuple(map(int,a.version.split('.'))) < (1,4,2) and coach.is_file():assets.append(coach)
    (dist/'SHA256SUMS.txt').write_text(''.join(f'{digest(p)}  {p.name}\n' for p in assets))
    print(json.dumps({'archive':str(archive),'sha256':digest(archive),'files':len(rows)+1,'bytes':archive.stat().st_size},indent=2))
if __name__=='__main__':main()
