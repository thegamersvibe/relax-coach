#!/usr/bin/env python3
"""Generate positive-use DATA only. Never execute or copy donor Lua code.

python -I -B tools/gen_cleanup_uses.py /path/KeepOrToss-1.7.0.zip [--check]
"""
import argparse, hashlib, re, sys, zipfile
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parent))
from lua_literals import Parser, ParseError, tokenize

ROOT=Path(__file__).resolve().parents[1]
OUTPUT=ROOT/'addon/RelaxQuestPlanner/Core/CleanupUses.lua'
ZIP_SHA='03910d502a00f6912cb372f946873908ed7c9ed6f6261c5c645a49991a6fb33d'
URL='https://mediafilez.forgecdn.net/files/9575/258/KeepOrToss-1.7.0.zip'
PREAMBLE='local _, ns = ...\n'
SOURCES={'reagent':['Reagents.lua'],'teaches':['Reagents.lua'],'tool':['Tools.lua'],
         'spellReagent':['SpellReagents.lua'],'openable':['Openable.lua','Locks.lua'],
         'crate':['Crates.lua'],'petFood':['PetFood.lua'],'food':['Food.lua'],
         'bonus':['Bonuses.lua'],'known':['KnownUses.lua'],'starts':['Starts.lua'],'vanilla':['Vanilla.lua']}
COUNTS={'reagent':594,'teaches':1978,'tool':32,'spellReagent':23,'openable':522,'crate':127,
        'petFood':411,'food':362,'bonus':14,'known':53,'starts':554,'vanilla':1764}


def parse(text):
    return Parser(tokenize(text),'literal data').run()


def literal(value):
    if isinstance(value,bool): return str(value).lower()
    if isinstance(value,int): return str(value)
    if isinstance(value,(tuple,list)): return '{'+','.join(map(literal,value))+'}'
    if isinstance(value,dict):
        return '{'+','.join('['+literal(k)+']='+literal(v) for k,v in sorted(value.items()))+'}'
    raise ValueError('unexpected generated value')


def validate(text):
    if not text.startswith(PREAMBLE): raise ValueError('namespace binding missing')
    body=text[len(PREAMBLE):]
    tokens=tokenize(body)
    if any(t[0]=='str' or (t[0]=='name' and t[1]=='local') for t in tokens):
        raise ValueError('generated body must contain literals only')
    blocks=parse(body)
    if {k:len(v) for k,v in blocks.items()}!=COUNTS: raise ValueError('block counts changed')
    for name,files in SOURCES.items():
        match=re.search(r'(-- BEGIN '+name+r'\n.*?)(?=ns\.'+name+r' =)',text,re.S)
        if not match: raise ValueError('missing header '+name)
        header=match[1]
        for required in ['GENERATED, do not edit','Keep or Toss: Forever 1.7.0',ZIP_SHA]:
            if required not in header: raise ValueError('incomplete header '+name)
        for file in files:
            if not re.search(r'-- Source: Data/'+re.escape(file)+r'; sha256 [0-9a-f]{64}; build \d+\.\d+\.\d+\.\d+\n',header):
                raise ValueError('incomplete source '+file)
    vanilla=blocks['vanilla']; last=0; total=0
    for i in range(1,1765,2):
        a,b=vanilla[i],vanilla[i+1]
        if not last<a<=b: raise ValueError('vanilla range order')
        total+=b-a+1; last=b
    if total!=17599: raise ValueError('vanilla cardinality')
    if sum(v==1 for v in blocks['crate'].values())!=31: raise ValueError('crate count')
    if sum(v==2 for v in blocks['crate'].values())!=96: raise ValueError('crate filling count')
    if sum(bool(v[2]) for v in blocks['known'].values())!=27: raise ValueError('secret count')
    return blocks


def generate(archive):
    raw=Path(archive).read_bytes()
    if hashlib.sha256(raw).hexdigest()!=ZIP_SHA: raise ValueError('archive hash mismatch')
    tables={}; metadata={}
    with zipfile.ZipFile(archive) as z:
        for info in sorted(z.infolist(),key=lambda x:x.filename):
            if not info.filename.startswith('KeepOrToss/Data/') or not info.filename.endswith('.lua'): continue
            if info.file_size>2_000_000: raise ValueError('oversize data file')
            data=z.read(info); text=data.decode('utf-8')
            name=Path(info.filename).name
            build=re.search(r'build (\d+\.\d+\.\d+\.\d+)',text)
            if not build or name in metadata: raise ValueError('bad source metadata')
            metadata[name]=(hashlib.sha256(data).hexdigest(),build[1])
            for key,value in parse(text).items():
                if key in tables: raise ValueError('duplicate table '+key)
                tables[key]=value
    if len(metadata)!=15: raise ValueError('expected 15 data files')
    if len(tables['OPENABLE_ITEMS'])!=522 or len(tables['LOCKPICKING'])!=23:
        raise ValueError('source openable/lock counts changed')
    recipes=tables['RECIPES']
    reagent={item:sorted({recipes[spell][1] for spell in refs.values()}) for item,refs in tables['REAGENT_RECIPES'].items()}
    teaches={}
    for row in recipes.values():
        if row[3]:
            if row[3] in teaches and teaches[row[3]]!=row[1]: raise ValueError('conflicting profession')
            teaches[row[3]]=row[1]
    def presence(key): return {k:True for k in tables[key]}
    crates={k:1 for k in tables['CRATES']}
    for row in tables['CRATES'].values():
        for filling in row.values():
            if crates.get(filling[1])==1: raise ValueError('crate/filling overlap')
            crates[filling[1]]=2
    blocks={'reagent':reagent,'teaches':teaches,'tool':presence('TOOLS'),
            'spellReagent':presence('SPELL_REAGENTS'),
            'openable':presence('OPENABLE_ITEMS')|presence('LOCKPICKING'),
            'crate':crates,'petFood':presence('PET_FOODS'),'food':presence('FOOD'),
            'bonus':presence('CREATURE_BONUSES'),
            'known':{k:[v.get('classID',0),bool(v.get('secret'))] for k,v in tables['KNOWN_USES'].items()},
            'starts':tables['STARTS_QUEST'],'vanilla':tables['VANILLA_ITEMS']}
    out=[PREAMBLE.rstrip(),'-- Positive-use evidence only; absence proves nothing. Credit: THIRD_PARTY.md.']
    for name,files in SOURCES.items():
        out+=['','-- BEGIN '+name,'-- GENERATED, do not edit; tools/gen_cleanup_uses.py',
              '-- Package: Keep or Toss: Forever 1.7.0; Aquary; data tables only, no code',
              '-- Zip sha256: '+ZIP_SHA]
        for file in files:
            sha,build=metadata[file]
            out.append(f'-- Source: Data/{file}; sha256 {sha}; build {build}')
        out.append('ns.'+name+' = {')
        for item,value in sorted(blocks[name].items()): out.append(f'    [{item}] = {literal(value)},')
        out.append('}')
    text='\n'.join(out)+'\n'; validate(text); return text


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('zip',type=Path); ap.add_argument('--check',action='store_true')
    a=ap.parse_args(); text=generate(a.zip)
    if a.check:
        if OUTPUT.read_bytes()!=text.encode(): raise SystemExit('generated data differs')
        credit=(ROOT/'THIRD_PARTY.md').read_text()
        if 'Keep or Toss: Forever 1.7.0' not in credit or 'data tables only, no code' not in credit:
            raise SystemExit('missing third-party credit')
        print('PASS: pinned hash, literal-only blocks, provenance, counts, byte-identical regeneration')
    else:
        OUTPUT.write_text(text); print('Generated',OUTPUT.relative_to(ROOT))

if __name__=='__main__': main()
