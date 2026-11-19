// A forwarding route's "Content to forward": presets on top, then the
// direction's groups, each with a tri-state box and "n of m", open only
// when mixed (decision docs/design/decisions/2026-11-20-fwd-more-types.md).
import { useEffect, useRef, useState } from 'react';
import { ChevronRight, TriangleAlert } from 'lucide-react';
import { CheckChips } from './forms.tsx';
import { groupsOf, ordered, presetOf, presetsOf, typeLabel, type Group } from './fwdtypes.ts';

export const EMPTY_TYPES = 'Choose at least one kind of message';

// A real checkbox that can be mixed: indeterminate is a DOM property only.
function TriBox({ on, mixed, label, onChange }: { on: boolean; mixed: boolean; label: string; onChange: () => void }) {
	const ref = useRef<HTMLInputElement>(null);
	useEffect(() => { if (ref.current) ref.current.indeterminate = mixed; }, [mixed]);
	return <input ref={ref} type="checkbox" checked={on} aria-checked={mixed ? 'mixed' : on} aria-label={label} onChange={onChange} />;
}

export function TypePicker<K extends string>({ options, value, onChange }: { options: readonly K[]; value: K[]; onChange: (v: K[]) => void }) {
	const groups = groupsOf(options);
	const presets = presetsOf(options);
	const preset = presetOf(options, value);
	// Opened or closed by hand; otherwise a group is open while mixed.
	const [open, setOpen] = useState<Record<string, boolean>>({});
	const set = (v: string[]) => onChange(ordered(options, v as K[]));
	const count = (g: Group) => g.types.filter((t) => value.includes(t as K)).length;
	const toggleGroup = (g: Group) => {
		const full = count(g) === g.types.length;
		set(full ? value.filter((t) => !g.types.includes(t)) : [...value, ...g.types.filter((t) => !value.includes(t as K))]);
	};
	return (
		<fieldset className="a-field a-typepicker">
			<legend className="a-field-label">Content to forward</legend>
			<div className="a-seg a-typepresets" role="radiogroup" aria-label="Preset">
				{presets.map((p) => (
					<button key={p.id} type="button" role="radio" aria-checked={preset?.id === p.id} className={preset?.id === p.id ? 'on' : ''} onClick={() => set(p.types)}>{p.name}</button>
				))}
				{!preset && <button type="button" role="radio" aria-checked className="on">Custom</button>}
			</div>
			<ul className="a-typegroups">
				{groups.map((g) => {
					const n = count(g);
					const single = !g.list && g.types.length === 1;
					const mixed = n > 0 && n < g.types.length;
					const isOpen = !single && (open[g.id] ?? mixed);
					return (
						<li key={g.id} className="a-typegroup">
							<div className="a-typegroup-head">
								<label className="a-typegroup-name">
									<TriBox on={n === g.types.length} mixed={mixed} label={g.name} onChange={() => toggleGroup(g)} />
									<span>{g.name}</span>
								</label>
								{g.hint && <span className="a-muted sm a-ellipsis">{g.hint}</span>}
								<span className="a-grow" />
								{!single && <span className="a-muted sm num">{n} of {g.types.length}</span>}
								{!single && (
									<button type="button" className="a-icon-btn a-typegroup-toggle" aria-expanded={isOpen} aria-label={`${isOpen ? 'Hide' : 'Show'} ${g.name}`} onClick={() => setOpen({ ...open, [g.id]: !isOpen })}>
										<ChevronRight size={15} />
									</button>
								)}
							</div>
							{isOpen && (
								<div className="a-typegroup-body">
									<CheckChips items={g.types.map((t) => [t as K, typeLabel(t)])} value={value} onChange={(v) => { setOpen({ ...open, [g.id]: true }); set(v); }} />
								</div>
							)}
						</li>
					);
				})}
			</ul>
			{!value.length && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{EMPTY_TYPES}.</span></div>}
			<span className="a-field-hint">Each part of a message is filtered on its own: a caption follows Text, a file its own type.</span>
		</fieldset>
	);
}
