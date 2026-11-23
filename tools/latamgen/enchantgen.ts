// Dumps latam-ro-calc's enchant table (EnchantTable, by AegisName, and the
// Malangdo weapons, by item ID) as JSON for internal/clientimport:
//
//	cp tools/latamgen/enchantgen.ts tmp/latam-ro-calc/ && cd tmp/latam-ro-calc && bun enchantgen.ts > ../../internal/clientimport/latam_enchants.json
//
// Positions are latam-ro-calc's enchant1..enchant4, the card indexes 0-3.
import { EnchantTable, getMalangdoEnchants } from './src/app/constants/enchant_item/_enchant_table';
import { MALANGDO_WEAPON_IDS } from './src/app/constants/enchant_item/malangdo_weapons';

const byName: Record<string, (string[] | null)[]> = {};
for (const e of EnchantTable) {
  byName[e.name] = e.enchants.map((p: any) => (p && p.length ? [...new Set(p.filter(Boolean))] as string[] : null));
}
const ids = [...new Set(Object.values(MALANGDO_WEAPON_IDS).flat())].sort((a, b) => a - b);
process.stdout.write(JSON.stringify({ byName, malangdo: { weapons: ids, pool: (getMalangdoEnchants(ids[0], 0) as any)[3] } }, null, 0));
