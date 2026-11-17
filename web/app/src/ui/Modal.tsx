// The one dialog pattern: centred, header with close, scrolling body,
// footer for actions. Escape or a backdrop click closes it.
import { useEffect, type ReactNode } from 'react';
import { X } from 'lucide-react';

export function Modal({ title, icon, onClose, children, footer, wide = false }: { title: string; icon?: ReactNode; onClose: () => void; children: ReactNode; footer?: ReactNode; wide?: boolean }) {
	useEffect(() => {
		const on = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
		addEventListener('keydown', on);
		const prev = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		return () => { removeEventListener('keydown', on); document.body.style.overflow = prev; };
	}, [onClose]);
	return (
		<div className="a-modal-wrap" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
			<div className={`a-modal ${wide ? 'wide' : ''}`} role="dialog" aria-modal="true" aria-label={title}>
				<header className="a-modal-head">
					{icon}
					<b className="a-grow a-ellipsis">{title}</b>
					<button type="button" className="a-icon-btn" onClick={onClose} aria-label="Close"><X size={16} /></button>
				</header>
				<div className="a-modal-body a-form">{children}</div>
				{footer && <footer className="a-modal-foot">{footer}</footer>}
			</div>
		</div>
	);
}
