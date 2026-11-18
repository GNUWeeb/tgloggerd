// lottie-web's light ESM build ships no types of its own; it is the
// same player as the package's main entry.
declare module 'lottie-web/build/player/esm/lottie_light.min.js' {
	import lottie from 'lottie-web';
	export default lottie;
}
