#!/usr/bin/env python3
"""Build the authorized combined guild archive. No network or publication."""
from pathlib import Path
import argparse,hashlib,json,re,shutil,zipfile
ROOT=Path(__file__).resolve().parents[1]
REACTION_AUDIO={'Interface/AddOns/ForeverCoach/Media/Reactions/'+name+'.mp3' for name in ('level_up','bags_1','bags_2','death','campfire')}

def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()

def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('--exe',type=Path,required=True)
    ap.add_argument('--version',default='1.5.5')
    a=ap.parse_args()
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+',a.version):raise SystemExit('Invalid version')
    if not a.exe.is_file() or a.exe.read_bytes()[:2]!=b'MZ':raise SystemExit('Provide the verified Windows EXE')
    # From 1.5.0 the bundle also carries Relax Quest Planner under a new asset
    # name, so apps before 1.5.0 (which refuse that folder) ask for Update App.
    bundle=tuple(map(int,a.version.split('.')))>=(1,5,0)
    addons=[('ForeverCoach','Relax Coach'),('Journey','Relax Journey')]+([('RelaxQuestPlanner','Relax Quest Planner')] if bundle else [])
    for folder,title in addons:
        if '## Title: '+title not in (ROOT/'addon'/folder/(folder+'.toc')).read_text():raise SystemExit('Unexpected addon identity')
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
    for p,name in inputs:
        parts=name.split('/')
        if p.is_symlink() or any(parent.is_symlink() for parent in p.parents):raise SystemExit('Symlink input rejected: '+name)
        if any(x.lower() in {'wtf','private','evidence','backups','.git','__pycache__'} for x in parts):raise SystemExit('Private path rejected: '+name)
        if name.startswith('/') or '..' in parts or '\\' in name or ':' in name or name in names:raise SystemExit('Unsafe or duplicate path: '+name)
        if name.startswith('Interface/Content/') and p.suffix!='.html':raise SystemExit('Unexpected guide file: '+name)
        for root,toc in (('Interface/AddOns/Journey/','Journey.toc'),('Interface/AddOns/RelaxQuestPlanner/','RelaxQuestPlanner.toc')):
            if name.startswith(root) and name.lower().endswith('.toc') and name!=root+toc:raise SystemExit('Extra addon TOC rejected: '+name)
        reaction_audio=name in REACTION_AUDIO and tuple(map(int,a.version.split('.'))) >= (1,4,3)
        if name.startswith('Interface/AddOns/') and not reaction_audio and p.suffix.lower() not in {'.lua','.toc','.tga','.blp','.xml','.txt','.md','.ttf','.otf','.html','.png'}:raise SystemExit('Unexpected addon file: '+name)
        names.add(name)
    rows=[{'path':name,'sha256':digest(p),'bytes':p.stat().st_size} for p,name in inputs]
    manifest={'project':'Relax Coach','kind':'guild-combined','version':a.version,'journeyVersion':'4.00.04','contentVersion':'4.00.08'}
    if planner_version:manifest['plannerVersion']=planner_version
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
    archive=dist/(f'RelaxCoach-Bundle-{a.version}.zip' if bundle else f'RelaxCoach-Guild-{a.version}.zip')
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
